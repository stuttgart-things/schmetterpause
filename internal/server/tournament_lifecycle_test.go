package server_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/stuttgart-things/schmetterpause/internal/domain"
)

// The close button used to appear only once every match was in, which left
// exactly the tournaments that most needed taking off the list — the ones
// nobody finishes — standing open forever.
func TestAnUnfinishedTournamentCanStillBeEnded(t *testing.T) {
	h, store := kioskHandler(t)
	field := seedField(t, store)
	id := seedTournamentMode(t, store, 3, 11, field)

	body := drawBodyAt(t, h, "/tournaments/"+id)
	if !strings.Contains(body, "/close") {
		t.Fatal("a tournament with no results offers no way to end it")
	}
	if !strings.Contains(body, "Es fehlen noch Spiele") {
		t.Error("ending an unfinished tournament does not say what it means")
	}

	rec := kioskPost(t, h, "/tournaments/"+id+"/close", nil, url.Values{})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("closing = %d, want 303", rec.Code)
	}

	list := listBody(t, h, "/tournaments")
	if !strings.Contains(list, "vergangene Turniere") {
		t.Error("the ended tournament did not reach the history")
	}
}

// Running and past answer different questions, so they are different lists.
func TestTheListSeparatesRunningFromPast(t *testing.T) {
	h, store := kioskHandler(t)
	field := seedField(t, store)
	open := seedTournamentMode(t, store, 3, 11, field)
	done := seedTournamentClosed(t, store, field)

	body := listBody(t, h, "/tournaments")
	running := body[:strings.Index(body, "vergangene Turniere")]

	if !strings.Contains(running, open) {
		t.Error("the running tournament is not in the running list")
	}
	if strings.Contains(running, done) {
		t.Error("a finished tournament sits above the fold")
	}
	if !strings.Contains(body, done) {
		t.Error("the finished tournament is nowhere at all")
	}
}

// Deleting is for a typo, not for an evening. A bracket with results in it
// would leave them behind as casual matches — still rated, and back inside
// the measurement they were taken out of.
func TestOnlyAnEmptyTournamentIsDeleted(t *testing.T) {
	h, store, cookie := twoPlayers(t)
	anna, bodo := playerIDs(t, store, "Anna", "Bodo")
	field := []uuid.UUID{anna, bodo}

	empty := seedTournamentMode(t, store, 3, 11, field)
	if rec := postAs(t, h, "/tournaments/"+empty+"/delete", cookie, url.Values{}); rec.Code != http.StatusSeeOther {
		t.Fatalf("deleting an empty tournament = %d, want 303", rec.Code)
	}
	if _, err := store.tournaments.ByID(t.Context(), mustUUID(t, empty)); err == nil {
		t.Error("the empty tournament is still there")
	}

	played := seedTournamentMode(t, store, 3, 11, field)
	enterOne(t, h, cookie, played, anna, bodo)

	if rec := postAs(t, h, "/tournaments/"+played+"/delete", cookie, url.Values{}); rec.Code != http.StatusSeeOther {
		t.Fatalf("deleting a played tournament = %d, want 303", rec.Code)
	}
	if _, err := store.tournaments.ByID(t.Context(), mustUUID(t, played)); err != nil {
		t.Error("a tournament with a result was deleted")
	}
	if n := len(store.matches.all()); n != 1 {
		t.Errorf("got %d matches, want the result kept", n)
	}
}

// The field is the draw. Changing it after a result exists would move later
// pairings into slots their results were not played in.
func TestATournamentIsEditableUntilSomethingIsPlayed(t *testing.T) {
	h, store, cookie := twoPlayers(t)
	anna, bodo := playerIDs(t, store, "Anna", "Bodo")
	cesar, err := store.Players().Create(t.Context(), "Cesar", domain.DefaultTTR)
	if err != nil {
		t.Fatalf("creating Cesar: %v", err)
	}
	id := seedTournamentMode(t, store, 3, 11, []uuid.UUID{anna, bodo})

	if body := signedInDraw(t, h, cookie, id); !strings.Contains(body, "Turnier ändern") {
		t.Fatal("an untouched tournament offers no way to change it")
	}

	edit := url.Values{
		"name": {"Umbenannt"}, "best_of": {"1"}, "points_to_win": {"21"},
		"format":    {string(domain.TournamentDoubleRoundRobin)},
		"player_id": {anna.String(), bodo.String(), cesar.ID.String()},
	}
	if rec := postAs(t, h, "/tournaments/"+id+"/edit", cookie, edit); rec.Code != http.StatusSeeOther {
		t.Fatalf("editing = %d, want 303", rec.Code)
	}

	after, err := store.tournaments.ByID(t.Context(), mustUUID(t, id))
	if err != nil {
		t.Fatalf("ByID: %v", err)
	}
	switch {
	case after.Name != "Umbenannt":
		t.Error("the name did not change")
	case len(after.Players) != 3:
		t.Errorf("the field has %d players, want 3", len(after.Players))
	case after.BestOf != 1 || after.PointsToWin != 21:
		t.Errorf("the mode is best of %d to %d, want 1 to 21", after.BestOf, after.PointsToWin)
	case after.Format != domain.TournamentDoubleRoundRobin:
		t.Errorf("the format is %q, want a return leg", after.Format)
	}

}

// Once a result exists the draw is settled: the offer disappears and the
// endpoint refuses, because a field that moved afterwards would put later
// pairings in slots their results were not played in.
func TestAPlayedTournamentIsNoLongerEditable(t *testing.T) {
	h, store, cookie := twoPlayers(t)
	anna, bodo := playerIDs(t, store, "Anna", "Bodo")
	id := seedTournamentMode(t, store, 3, 11, []uuid.UUID{anna, bodo})

	enterOne(t, h, cookie, id, anna, bodo)

	if body := signedInDraw(t, h, cookie, id); strings.Contains(body, "Turnier ändern") {
		t.Error("a played tournament still offers to move its field")
	}
	rec := postAs(t, h, "/tournaments/"+id+"/edit", cookie, url.Values{
		"name": {"Zu spät"}, "best_of": {"3"}, "points_to_win": {"11"},
		"player_id": {anna.String(), bodo.String()},
	})
	if !strings.Contains(rec.Header().Get("Location"), "fehler=") {
		t.Error("a played tournament was edited anyway")
	}
	after, err := store.tournaments.ByID(t.Context(), mustUUID(t, id))
	if err != nil {
		t.Fatalf("ByID: %v", err)
	}
	if after.Name == "Zu spät" {
		t.Error("the name changed after a result existed")
	}
}

func enterOne(t *testing.T, h http.Handler, cookie *http.Cookie, id string, home, away uuid.UUID) {
	t.Helper()

	rec := postAs(t, h, "/matches", cookie, url.Values{
		"tournament_id": {id}, "tournament_round": {"1"},
		"home_id": {home.String()}, "away_id": {away.String()},
		"set_home_1": {"11"}, "set_away_1": {"5"},
		"set_home_2": {"11"}, "set_away_2": {"7"},
	})
	if loc := rec.Header().Get("Location"); strings.Contains(loc, "fehler=") {
		t.Fatalf("entering a result: %s", loc)
	}
}

func mustUUID(t *testing.T, s string) uuid.UUID {
	t.Helper()

	id, err := uuid.Parse(s)
	if err != nil {
		t.Fatalf("parsing %s: %v", s, err)
	}
	return id
}
