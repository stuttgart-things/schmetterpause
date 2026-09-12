package server_test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/stuttgart-things/schmetterpause/internal/auth"
	"github.com/stuttgart-things/schmetterpause/internal/domain"
)

// A tournament somebody is put into used to be invisible to them. Another
// player ticks a name, the draw exists, and the one person who never learned
// about it was the player now in it: the list lived one navigation step away
// under /tournaments, and nothing on the page they actually open said a word.
func TestTheStartPageSaysWhichTournamentYouAreIn(t *testing.T) {
	h, store, cookie := twoPlayers(t)
	anna, bodo := playerIDs(t, store, "Anna", "Bodo")
	seedOpenTournament(t, store, "Mittwochsturnier", time.Time{}, anna, bodo)

	body := fragment(t, h, "/", cookie).Body.String()

	for _, want := range []string{"Du bist dabei", "Mittwochsturnier", "du spielst mit"} {
		if !strings.Contains(body, want) {
			t.Errorf("the start page does not say %q: %s", want, body)
		}
	}
	if !strings.Contains(body, `href="/tournaments/`) {
		t.Errorf("the notice does not lead to the draw: %s", body)
	}
}

// Somebody who is not in it still gets to see that the table is busy — that
// is public on /tournaments — but the heading is about the office rather than
// about them, and no row is marked.
func TestATournamentYouAreNotInIsShownWithoutTheMark(t *testing.T) {
	h, store, cookie := twoPlayers(t)
	_, bodo := playerIDs(t, store, "Anna", "Bodo")
	cesar, err := store.Players().Create(t.Context(), "Cesar", domain.DefaultTTR)
	if err != nil {
		t.Fatalf("creating Cesar: %v", err)
	}
	seedOpenTournament(t, store, "Ohne Anna", time.Time{}, bodo, cesar.ID)

	body := fragment(t, h, "/", cookie).Body.String()

	if !strings.Contains(body, "Läuft gerade") {
		t.Errorf("the start page does not say that something is on: %s", body)
	}
	if strings.Contains(body, "du spielst mit") {
		t.Errorf("Anna is marked as playing in a tournament she is not in: %s", body)
	}
	if strings.Contains(body, "Du bist dabei") {
		t.Errorf("the notice claims Anna is in the draw: %s", body)
	}
}

// Closed is over. A notice about what is happening that keeps last month's
// evening on the start page stops being a notice.
func TestAClosedTournamentIsNotOnTheStartPage(t *testing.T) {
	h, store, cookie := twoPlayers(t)
	anna, bodo := playerIDs(t, store, "Anna", "Bodo")
	seedTournamentClosed(t, store, []uuid.UUID{anna, bodo})

	body := fragment(t, h, "/", cookie).Body.String()

	if strings.Contains(body, "Läuft gerade") || strings.Contains(body, "Du bist dabei") {
		t.Errorf("a closed tournament is announced as running: %s", body)
	}
}

// Nothing on means no card at all. A line saying "kein Turnier" would be one
// to read past on every day there is none, which is most of them.
func TestNoTournamentMeansNoNotice(t *testing.T) {
	h, _, cookie := twoPlayers(t)

	body := fragment(t, h, "/", cookie).Body.String()

	if !strings.Contains(body, `id="running"`) {
		t.Errorf("the section the swaps aim at is missing: %s", body)
	}
	if strings.Contains(body, "Läuft gerade") || strings.Contains(body, "Du bist dabei") {
		t.Errorf("an empty notice announced something: %s", body)
	}
}

// Two open tournaments, and only one of them is the reader's. Theirs comes
// first: the section exists to tell somebody where they are expected, and
// somebody else's evening above it is the thing it was meant to fix.
func TestYourOwnTournamentComesFirst(t *testing.T) {
	h, store, cookie := twoPlayers(t)
	anna, bodo := playerIDs(t, store, "Anna", "Bodo")
	cesar, err := store.Players().Create(t.Context(), "Cesar", domain.DefaultTTR)
	if err != nil {
		t.Fatalf("creating Cesar: %v", err)
	}

	// The newer one is the one Anna is not in, so the store hands it back
	// first and only the sort can put hers above it.
	seedOpenTournament(t, store, "Annas Runde", time.Now().Add(-time.Hour), anna, bodo)
	seedOpenTournament(t, store, "Fremde Runde", time.Now(), bodo, cesar.ID)

	body := fragment(t, h, "/", cookie).Body.String()

	mine, theirs := strings.Index(body, "Annas Runde"), strings.Index(body, "Fremde Runde")
	switch {
	case mine < 0 || theirs < 0:
		t.Fatalf("not both tournaments are on the page: %s", body)
	case mine > theirs:
		t.Errorf("the tournament Anna is in stands below the one she is not in: %s", body)
	}
}

// The fragment behind the poll. It is what makes the notice turn up on a
// phone lying beside the plate, rather than on a reload nobody had a reason
// to make — so it has to answer with the notice and without the page.
func TestTheRunningFragmentCarriesTheNoticeAlone(t *testing.T) {
	h, store, cookie := twoPlayers(t)
	anna, bodo := playerIDs(t, store, "Anna", "Bodo")
	seedOpenTournament(t, store, "Mittwochsturnier", time.Time{}, anna, bodo)

	rec := fragment(t, h, "/fragments/running", cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()

	if !strings.Contains(body, "Mittwochsturnier") {
		t.Errorf("the fragment does not carry the tournament: %s", body)
	}
	if strings.Contains(body, "<html") || strings.Contains(body, `class="mainnav"`) {
		t.Errorf("the fragment carries the whole page: %s", body)
	}
	// The element the poll hangs on stays where it is; the fragment is only
	// what goes inside it.
	if strings.Contains(body, `id="running"`) {
		t.Errorf("the fragment brings its own container: %s", body)
	}
}

// A reader nobody is recognised as sees what is on and nothing marked as
// theirs. Standing in front of the machine wondering whether the evening has
// started is exactly the moment this answers.
func TestTheNoticeShowsWithoutASession(t *testing.T) {
	h, store, _ := twoPlayers(t)
	anna, bodo := playerIDs(t, store, "Anna", "Bodo")
	seedOpenTournament(t, store, "Mittwochsturnier", time.Time{}, anna, bodo)

	body := fragment(t, h, "/", nil).Body.String()

	if !strings.Contains(body, "Mittwochsturnier") {
		t.Errorf("a stranger is not told that a tournament is on: %s", body)
	}
	if strings.Contains(body, "du spielst mit") {
		t.Errorf("a row is marked for a reader nobody is recognised as: %s", body)
	}
}

// Signing in has to bring the mark with it. The page behind the card was
// rendered for a reader nobody was recognised as, so a draw this player is in
// was sitting there without a word saying it was theirs — which is the exact
// thing the section was added to stop happening.
func TestSigningInMarksTheTournamentAsYours(t *testing.T) {
	store := newMemStore()
	h := newHandlerWith(store, auth.NewCookieAuthenticator(store.Identities(), testSessionKey, false))

	code := codeInPage.FindStringSubmatch(join(t, h, "Anna").Body.String())
	if code == nil {
		t.Fatal("joining showed no recovery code")
	}
	bodo, err := store.Players().Create(t.Context(), "Bodo", domain.DefaultTTR)
	if err != nil {
		t.Fatalf("creating Bodo: %v", err)
	}

	players, err := store.Players().List(t.Context())
	if err != nil {
		t.Fatalf("List(): %v", err)
	}
	var anna uuid.UUID
	for _, p := range players {
		if p.DisplayName == "Anna" {
			anna = p.ID
		}
	}
	seedOpenTournament(t, store, "Mittwochsturnier", time.Time{}, anna, bodo.ID)

	body := signIn(t, h, anna.String(), code[1]).Body.String()

	if !strings.Contains(body, `id="running" hx-swap-oob="innerHTML"`) {
		t.Fatalf("signing in does not refresh the tournament notice: %s", body)
	}
	if !strings.Contains(body, "du spielst mit") {
		t.Errorf("the refreshed notice does not mark Anna's tournament: %s", body)
	}
}

// seedOpenTournament puts an open tournament with the given field in the
// store. createdAt decides the order the store hands them back in, which is
// what the sorting test needs a handle on.
func seedOpenTournament(
	t *testing.T, store *memStore, name string, createdAt time.Time, field ...uuid.UUID,
) string {
	t.Helper()

	created, err := store.tournaments.Create(t.Context(), domain.Tournament{
		Name: name, Format: domain.TournamentRoundRobin,
		Status: domain.TournamentOpen, CreatedBy: field[0], CreatedAt: createdAt,
		BestOf: 3, PointsToWin: 11, Rated: true, Players: field,
	})
	if err != nil {
		t.Fatalf("seeding %q: %v", name, err)
	}
	return created.ID.String()
}
