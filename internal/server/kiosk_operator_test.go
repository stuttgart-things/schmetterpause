package server_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stuttgart-things/schmetterpause/internal/domain"
)

// seedPlayers puts names in the store and hands back their ids.
func seedPlayers(t *testing.T, store *memStore, names ...string) []string {
	t.Helper()

	ids := make([]string, 0, len(names))
	for _, name := range names {
		p, err := store.Players().Create(t.Context(), name, domain.DefaultTTR)
		if err != nil {
			t.Fatalf("seeding %s: %v", name, err)
		}
		ids = append(ids, p.ID.String())
	}
	return ids
}

// The state issue #90 is about: a machine that holds the token and nothing
// else. It used to be able to settle a result with no counter-party; now it
// cannot write at all until it says who is typing.
func TestAnUnnamedKioskCannotWrite(t *testing.T) {
	h, store := kioskHandler(t)
	cookie := unlockOnly(t, h)
	ids := seedPlayers(t, store, "Anna", "Bodo")

	writes := []struct {
		name string
		path string
		form url.Values
	}{
		{"a match", "/kiosk/matches", url.Values{
			"home_id": {ids[0]}, "away_id": {ids[1]},
			"best_of": {"1"}, "points_to_win": {"11"},
			"set_home_1": {"11"}, "set_away_1": {"7"},
		}},
		{"a player", "/kiosk/players", url.Values{"display_name": {"Clara"}}},
		{"a recovery code", "/kiosk/credentials", url.Values{"player_id": {ids[0]}}},
	}
	for _, w := range writes {
		rec := kioskPost(t, h, w.path, cookie, w.form)
		if rec.Code != http.StatusForbidden {
			t.Errorf("%s = %d, want 403 while nobody is named", w.name, rec.Code)
		}
	}

	n, err := store.Players().Count(t.Context())
	if err != nil {
		t.Fatalf("Count(): %v", err)
	}
	if n != 2 {
		t.Errorf("%d players, want the two seeded ones and nothing the kiosk wrote", n)
	}
}

// The unlocked machine shows the question and nothing else. Offering the
// entry form beside it would make answering look optional.
func TestAnUnnamedKioskAsksWhoIsTyping(t *testing.T) {
	h, _ := kioskHandler(t)
	cookie := unlockOnly(t, h)

	body := kioskBody(t, h, cookie)
	if !strings.Contains(body, "Wer trägt ein?") {
		t.Errorf("the machine does not ask who is typing: %s", body)
	}
	if strings.Contains(body, "Ergebnis eintragen") {
		t.Error("the entry form is offered before anybody is named")
	}
}

// The check the browser cookie could only approximate. There is no player
// session here at all — this is the private window that walked around #91 —
// and the refusal comes from the grant instead.
func TestTheOperatorCannotEnterTheirOwnMatch(t *testing.T) {
	h, store := kioskHandler(t)
	cookie := unlockOnly(t, h)
	ids := seedPlayers(t, store, "Anna", "Bodo")

	// Anna is at the laptop, and Anna is playing.
	rec := kioskPost(t, h, "/kiosk/operator", cookie, url.Values{"operator_id": {ids[0]}})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("naming Anna = %d: %s", rec.Code, rec.Body.String())
	}

	rec = kioskPost(t, h, "/kiosk/matches", cookie, url.Values{
		"home_id": {ids[0]}, "away_id": {ids[1]},
		"best_of": {"1"}, "points_to_win": {"11"},
		"set_home_1": {"11"}, "set_away_1": {"7"},
	})
	if !strings.Contains(rec.Body.String(), "Dein eigenes Spiel nicht hier") {
		t.Errorf("Anna scored a match she plays in: %d %s", rec.Code, rec.Body.String())
	}

	matches, err := store.Matches().Recent(t.Context(), 10)
	if err != nil {
		t.Fatalf("Recent(): %v", err)
	}
	if len(matches) != 0 {
		t.Errorf("%d matches were written, want none", len(matches))
	}
}

// reported_by used to be the home player, which made a kiosk row
// indistinguishable from a result that player entered themselves — the lie the
// Definition of Done in issue #43 had to work around. It names the operator.
func TestAKioskMatchIsCreditedToTheOperator(t *testing.T) {
	h, store := kioskHandler(t)
	cookie := unlockOnly(t, h)
	ids := seedPlayers(t, store, "Anna", "Bodo")
	operator := nameOperator(t, h, store, cookie)

	kioskEnter(t, h, cookie, ids[0], ids[1])

	matches, err := store.Matches().Recent(t.Context(), 10)
	if err != nil {
		t.Fatalf("Recent(): %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("%d matches, want 1", len(matches))
	}
	if matches[0].ReportedBy != operator {
		t.Errorf("ReportedBy = %s, want the operator %s — not the home player",
			matches[0].ReportedBy, operator)
	}
}

// The laptop is passed on during an evening, and the alternative to naming a
// second operator is unlocking the machine again.
func TestTheOperatorCanBeHandedOn(t *testing.T) {
	h, store := kioskHandler(t)
	cookie := unlockOnly(t, h)
	ids := seedPlayers(t, store, "Anna", "Bodo", "Clara")

	for _, id := range []string{ids[0], ids[2]} {
		rec := kioskPost(t, h, "/kiosk/operator", cookie, url.Values{"operator_id": {id}})
		if rec.Code != http.StatusSeeOther {
			t.Fatalf("naming %s = %d: %s", id, rec.Code, rec.Body.String())
		}
	}

	// Clara has it now, so Anna may be scored again and Clara may not.
	body := kioskBody(t, h, cookie)
	if !strings.Contains(body, "Clara") {
		t.Errorf("the page does not say who is typing: %s", body)
	}

	rec := kioskPost(t, h, "/kiosk/matches", cookie, url.Values{
		"home_id": {ids[2]}, "away_id": {ids[1]},
		"best_of": {"1"}, "points_to_win": {"11"},
		"set_home_1": {"11"}, "set_away_1": {"7"},
	})
	if !strings.Contains(rec.Body.String(), "Dein eigenes Spiel nicht hier") {
		t.Errorf("Clara scored her own match after taking over: %s", rec.Body.String())
	}
}

// A machine somebody took back from /admin must not be quietly given an
// operator and put back to work.
func TestARevokedMachineCannotBeNamed(t *testing.T) {
	h, store := kioskHandler(t)
	cookie := unlockOnly(t, h)
	ids := seedPlayers(t, store, "Anna")

	if _, err := store.KioskGrants().RevokeAll(t.Context(), time.Now()); err != nil {
		t.Fatalf("RevokeAll(): %v", err)
	}

	rec := kioskPost(t, h, "/kiosk/operator", cookie, url.Values{"operator_id": {ids[0]}})
	if rec.Code != http.StatusForbidden {
		t.Errorf("naming a revoked machine = %d, want 403: %s", rec.Code, rec.Body.String())
	}
}

// Nobody is named by a form that carries no name, and the refusal comes back
// as the question rather than as an error page.
func TestNamingNeedsAPlayer(t *testing.T) {
	h, _ := kioskHandler(t)
	cookie := unlockOnly(t, h)

	for _, value := range []string{"", "not-a-uuid", "3f9d3b1e-0000-4000-8000-000000000000"} {
		rec := kioskPost(t, h, "/kiosk/operator", cookie, url.Values{"operator_id": {value}})
		if rec.Code != http.StatusUnprocessableEntity {
			t.Errorf("operator_id=%q = %d, want 422", value, rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "Wer trägt ein?") {
			t.Errorf("operator_id=%q does not come back as the question", value)
		}
	}
}

// "Who scored this match" is the question the room used to answer. /admin
// answers it now, per machine.
func TestTheAdminPageNamesTheOperator(t *testing.T) {
	srv, store := adminAndKiosk(t)
	h := srv.Handler()

	annaCookie := sessionCookie(t, join(t, h, "Anna"))
	srv.GrantBootstrapAdmin(t.Context())

	cookie := unlockOnly(t, h)
	if body := getWith(t, h, "/admin", annaCookie).Body.String(); !strings.Contains(body, "noch niemand") {
		t.Errorf("an unnamed machine is not shown as unnamed: %s", body)
	}

	nameOperator(t, h, store, cookie)
	body := getWith(t, h, "/admin", annaCookie).Body.String()
	if !strings.Contains(body, scorekeeperName) {
		t.Errorf("the admin page does not name the operator: %s", body)
	}
}
