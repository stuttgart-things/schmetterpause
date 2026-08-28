package server

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/google/uuid"

	"github.com/stuttgart-things/schmetterpause/internal/auth"
	"github.com/stuttgart-things/schmetterpause/internal/credential"
	"github.com/stuttgart-things/schmetterpause/internal/domain"
	"github.com/stuttgart-things/schmetterpause/internal/ratelimit"
	"github.com/stuttgart-things/schmetterpause/internal/templates"
)

// The brake on the sign-in form, and a shipping condition rather than
// follow-up work (docs/adr/0007). Both dimensions grow the wait instead of
// slamming a door, because the same ADR forbids a limit that can lock a
// player out for good — that would rebuild issue #70 by another route.
var (
	// Per player. Three free tries, because mistyping a sixteen-character
	// code is what the person this exists for actually does. Past that the
	// wait doubles to five minutes, and working through six digits at five
	// minutes a guess takes years.
	signInPlayerPolicy = ratelimit.Policy{
		Free:   3,
		Step:   2 * time.Second,
		Max:    5 * time.Minute,
		Forget: time.Hour,
	}
	// Per address, and deliberately gentler. Its job is to stop one machine
	// walking the roster, not to police a person.
	//
	// The cap is low on purpose. Behind a proxy every request in the building
	// arrives from the same address, and a strict ceiling would then take the
	// whole office out — which is the failure this must not have. Thirty
	// seconds degrades to a short pause in that case while still leaving
	// guessing hopeless, and the per-player half is the one carrying the
	// weight anyway.
	signInAddressPolicy = ratelimit.Policy{
		Free:   15,
		Step:   time.Second,
		Max:    30 * time.Second,
		Forget: 15 * time.Minute,
	}
)

// signInRefused is the answer to every failed attempt, whichever half was
// wrong: an unknown player, a player with no credentials at all, a wrong PIN,
// a wrong code.
//
// One wording on purpose. "Dieser Spieler hat keine PIN" would tell whoever
// asked something about somebody else's account, and the door the recovery
// code uses is the same one (docs/adr/0007).
const signInRefused = "Das passt nicht. Prüf die PIN oder den Wiederherstellungscode."

// handleSignInForm serves the sign-in form on its own, so the start page can
// swap the join form for it without a reload.
func (s *Server) handleSignInForm(w http.ResponseWriter, r *http.Request) {
	view, err := s.signInView(r.Context(), uuid.Nil)
	if err != nil {
		s.log.ErrorContext(r.Context(), "loading the player list failed", "error", err)
		http.Error(w, "Anmeldung gerade nicht möglich", http.StatusInternalServerError)
		return
	}
	s.render(w, r, templates.SignIn(view))
}

// handleJoinForm serves the join form on its own, which is the way back from
// the sign-in form for somebody who is not on the list after all.
func (s *Server) handleJoinForm(w http.ResponseWriter, r *http.Request) {
	s.render(w, r, templates.Session(templates.SessionView{}))
}

// handleSignIn puts a browser back in touch with a player it can prove it is.
//
// This is the way back out of issue #70: a browser with no cookie used to be
// a stranger for good, because joining under a name that exists is refused
// and joining was the only route to a session.
//
// Name first, secret second (docs/adr/0007). With a salt per row there is no
// way to find a credential from the secret alone, and the player list is
// public regardless — the ranking, /matches and the sheet on the wall all
// name everybody.
func (s *Server) handleSignIn(w http.ResponseWriter, r *http.Request) {
	// The address is checked before anything is parsed, looked up or hashed.
	// Argon2id is deliberately expensive — 64 MiB a run — so a form nobody
	// checks first is not only a way to guess, it is a way to spend the
	// machine's memory.
	address := requestAddress(r)
	if wait := s.signInByAddress.Retry(address); wait > 0 {
		s.refuseForNow(w, r, uuid.Nil, wait)
		return
	}

	playerID, err := uuid.Parse(strings.TrimSpace(r.FormValue("player_id")))
	if err != nil {
		s.rejectSignIn(w, r, uuid.Nil, "Wähl erst deinen Namen.")
		return
	}

	if wait := s.signInByPlayer.Retry(playerID.String()); wait > 0 {
		s.refuseForNow(w, r, playerID, wait)
		return
	}

	secret := strings.TrimSpace(r.FormValue("secret"))
	if secret == "" {
		s.rejectSignIn(w, r, playerID, "Ohne PIN oder Code geht es nicht.")
		return
	}

	player, err := s.store.Players().ByID(r.Context(), playerID)
	switch {
	case errors.Is(err, domain.ErrNotFound):
		// Only the address is charged for a name nobody has. Counting an
		// id that resolves to nothing would let anybody fill the map with
		// invented ones.
		s.signInByAddress.Failed(address)
		s.rejectSignIn(w, r, uuid.Nil, signInRefused)
		return
	case err != nil:
		s.log.ErrorContext(r.Context(), "loading the player failed", "player_id", playerID, "error", err)
		s.rejectSignIn(w, r, playerID, "Das hat gerade nicht geklappt. Versuch es noch einmal.")
		return
	}

	ok, err := s.secretMatches(r.Context(), playerID, secret)
	if err != nil {
		// A database that is down is not a wrong guess, so nothing is
		// charged for it. Otherwise an outage would leave everybody waiting
		// once it came back.
		s.log.ErrorContext(r.Context(), "checking the credential failed", "player_id", playerID, "error", err)
		s.rejectSignIn(w, r, playerID, "Das hat gerade nicht geklappt. Versuch es noch einmal.")
		return
	}
	if !ok {
		s.signInByPlayer.Failed(playerID.String())
		s.signInByAddress.Failed(address)
		s.log.InfoContext(r.Context(), "sign-in refused", "player_id", playerID, "address", address)
		s.rejectSignIn(w, r, playerID, signInRefused)
		return
	}

	// Only the player half is cleared. Clearing the address half too would
	// hand anybody with an account of their own a way to reset the budget
	// every few guesses — and the address wait tops out at thirty seconds,
	// so leaving it standing costs a legitimate browser very little.
	s.signInByPlayer.Succeeded(playerID.String())

	// A new subject rather than the one this player already has somewhere.
	// A player holds several identities by design (docs/adr/0003), so signing
	// in on a new phone leaves the browser at home signed in — which is what
	// somebody who still has both would expect.
	subject := auth.NewSubject()
	if err := s.store.Identities().Link(r.Context(), domain.ProviderLocal, subject, playerID); err != nil {
		s.log.ErrorContext(r.Context(), "linking the new session failed", "player_id", playerID, "error", err)
		s.rejectSignIn(w, r, playerID, "Das hat gerade nicht geklappt. Versuch es noch einmal.")
		return
	}

	s.auth.SetCookie(w, subject)
	s.log.InfoContext(r.Context(), "player signed in", "player_id", playerID)

	s.renderSession(w, r, player, templates.SignedIn(templates.SessionView{
		DisplayName: player.DisplayName,
		PlayerID:    player.ID.String(),
	}))
}

// refuseForNow answers an attempt the brake is holding back.
//
// It says how long is left rather than only that something went wrong.
// "Versuch es später" is what a dead end sounds like, and this form exists
// because of a dead end.
func (s *Server) refuseForNow(w http.ResponseWriter, r *http.Request, selected uuid.UUID, wait time.Duration) {
	view, err := s.signInView(r.Context(), selected)
	if err != nil {
		s.log.ErrorContext(r.Context(), "loading the player list failed", "error", err)
		http.Error(w, "Anmeldung gerade nicht möglich", http.StatusInternalServerError)
		return
	}
	view.Error = "Zu viele Fehlversuche. Probier es in " + waitInWords(wait) + " noch einmal."

	// Retry-After is in seconds and has to be at least one, or a client that
	// reads it learns nothing.
	w.Header().Set("Retry-After", strconv.Itoa(max(int(wait.Round(time.Second)/time.Second), 1)))
	// 429 rather than 422: the request was fine, there have just been too
	// many of them. web/static/js/app.js knows to swap this one too.
	w.WriteHeader(http.StatusTooManyRequests)
	s.render(w, r, templates.SignIn(view))
}

// waitInWords puts a duration in the words somebody would use for it. Rounded
// up, because being told to wait five seconds and finding out at five that it
// was really six is worse than being told six.
func waitInWords(d time.Duration) string {
	if d < time.Minute {
		seconds := int((d + time.Second - 1) / time.Second)
		if seconds <= 1 {
			return "einer Sekunde"
		}
		return strconv.Itoa(seconds) + " Sekunden"
	}

	minutes := int((d + time.Minute - 1) / time.Minute)
	if minutes <= 1 {
		return "einer Minute"
	}
	return strconv.Itoa(minutes) + " Minuten"
}

// requestAddress is the address an attempt came from.
//
// RemoteAddr and nothing else. X-Forwarded-For is not read, because a header
// nobody verified is a header anybody sets — and a limit keyed on a value the
// caller chooses is not a limit. Behind a proxy this makes every request in
// the building one address, which the gentle per-address policy is chosen for;
// doing better needs a trusted-proxy setting, and that belongs with the
// cluster work in issue #89.
func requestAddress(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// secretMatches reports whether typed is one of this player's credentials.
//
// Both kinds are tried, because the form takes either and does not ask which
// one this is. The order follows the shape of what was typed rather than a
// preference: digits alone cannot be a recovery code, and anything with a
// letter in it cannot be a PIN — so the likely kind goes first and the second
// Argon2id run usually does not happen.
//
// A player with no credentials at all answers faster than one with a code,
// because there is nothing to hash. That timing says whether somebody has a
// way in — knowingly accepted: the player list is public either way, and
// under the threat model in docs/adr/0004 the difference does not buy
// anything a look at the ranking does not.
func (s *Server) secretMatches(ctx context.Context, playerID uuid.UUID, typed string) (bool, error) {
	attempts := []struct {
		kind   domain.CredentialKind
		secret string
	}{
		{domain.CredentialPIN, typed},
		// What somebody types is rarely what was printed: lower case, spaces
		// instead of hyphens, an O where a zero stands.
		{domain.CredentialRecovery, credential.NormalizeCode(typed)},
	}
	if !isDigits(typed) {
		attempts[0], attempts[1] = attempts[1], attempts[0]
	}

	for _, a := range attempts {
		if a.secret == "" {
			continue
		}

		stored, err := s.store.Credentials().ForPlayer(ctx, playerID, a.kind)
		switch {
		case errors.Is(err, domain.ErrNotFound):
			// Having no PIN is the ordinary state — setting one is optional.
			continue
		case err != nil:
			return false, fmt.Errorf("load %s credential: %w", a.kind, err)
		}

		ok, err := credential.Verify(stored.Hash, a.secret)
		if err != nil {
			// A row this build cannot read is not a wrong secret. It is
			// logged and the other kind still gets its turn, because
			// refusing everybody over one broken row would rebuild #70.
			s.log.ErrorContext(ctx, "stored credential is unreadable",
				"player_id", playerID, "kind", a.kind, "error", err)
			continue
		}
		if ok {
			return true, nil
		}
	}
	return false, nil
}

// isDigits reports whether s is nothing but digits, and not empty.
func isDigits(s string) bool {
	if s == "" {
		return false
	}
	return !strings.ContainsFunc(s, func(r rune) bool { return r < '0' || r > '9' })
}

// rejectSignIn re-renders the form with the reason, keeping the chosen name
// so nobody has to find themselves in the list twice.
func (s *Server) rejectSignIn(w http.ResponseWriter, r *http.Request, selected uuid.UUID, msg string) {
	view, err := s.signInView(r.Context(), selected)
	if err != nil {
		s.log.ErrorContext(r.Context(), "loading the player list failed", "error", err)
		http.Error(w, "Anmeldung gerade nicht möglich", http.StatusInternalServerError)
		return
	}
	view.Error = msg

	// 422 rather than 401: the request was well formed, what was in it was
	// not, and this is the same shape every other rejected form on this page
	// takes. HTMX swaps 4xx only because web/static/js/app.js says so.
	w.WriteHeader(http.StatusUnprocessableEntity)
	s.render(w, r, templates.SignIn(view))
}

// signInView loads the picker, by name.
//
// Alphabetical rather than by rating: somebody signing in is looking for
// themselves, and the ranking order is only useful to somebody reading the
// ranking.
func (s *Server) signInView(ctx context.Context, selected uuid.UUID) (templates.SignInView, error) {
	players, err := s.store.Players().List(ctx)
	if err != nil {
		return templates.SignInView{}, fmt.Errorf("load players: %w", err)
	}

	options := make([]templates.OpponentOption, 0, len(players))
	for _, p := range players {
		options = append(options, templates.OpponentOption{
			ID:          p.ID.String(),
			DisplayName: p.DisplayName,
			Selected:    p.ID == selected,
		})
	}
	slices.SortFunc(options, func(a, b templates.OpponentOption) int {
		return strings.Compare(
			strings.ToLower(a.DisplayName), strings.ToLower(b.DisplayName))
	})

	return templates.SignInView{Players: options}, nil
}

// renderSession writes what follows somebody arriving in a session, whether
// they just joined or just signed in: the region they acted in, and the
// places on the page that now say something different.
//
// Out of band rather than by reload. The region replaces itself, and a page
// that only catches up on the next reload leaves somebody with nowhere to
// enter a result and nothing saying so — see issue #75.
func (s *Server) renderSession(w http.ResponseWriter, r *http.Request, player domain.Player, region templ.Component) {
	signedIn := auth.WithPlayerID(r.Context(), player.ID)
	view := templates.SessionView{DisplayName: player.DisplayName, PlayerID: player.ID.String()}

	table, err := s.standingsView(signedIn)
	if err != nil {
		// The player is signed in; only the ranking is missing. Saying so
		// beats discarding what just succeeded.
		s.log.ErrorContext(r.Context(), "loading the standings failed", "error", err)
	}

	s.render(w, r, region)
	s.render(w, r, templates.WhoamiOOB(s.headerView(signedIn)))
	s.render(w, r, templates.PageHeadOOB(view))
	s.render(w, r, templates.StandingsOOB(table))

	// Both of these matter more after a sign-in than after a join. Somebody
	// who just got back to their player may have results waiting on them from
	// the days their browser had forgotten who they were.
	pending, err := s.pendingListView(signedIn, player.ID)
	if err != nil {
		s.log.ErrorContext(r.Context(), "loading the pending matches failed", "error", err)
	} else {
		s.render(w, r, templates.PendingListOOB(pending))
	}

	opponents, err := s.opponentOptions(signedIn, player.ID, uuid.Nil)
	if err != nil {
		// Same trade as the standings above. A reload brings the form;
		// discarding what succeeded would bring nothing.
		s.log.ErrorContext(r.Context(), "loading the opponents failed", "error", err)
		return
	}
	s.render(w, r, templates.MatchFormOOB(templates.NewMatchFormView(opponents)))
}
