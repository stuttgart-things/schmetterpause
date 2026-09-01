package server_test

import (
	"net/url"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/stuttgart-things/schmetterpause/internal/domain"
)

// A return leg is the same pair in a different slot, so the draw shows both
// and each gets its own entry form. Keyed on the pair alone the second one
// would show the first one's result (docs/adr/0011).
func TestAReturnLegIsItsOwnMatch(t *testing.T) {
	h, store := kioskHandler(t)
	cookie := unlock(t, h)
	field := seedField(t, store)
	id := seedFormat(t, store, domain.TournamentDoubleRoundRobin, false, field)

	body := drawBody(t, h, cookie, id)
	if got := strings.Count(body, `name="tournament_round"`); got != 2 {
		t.Fatalf("the draw offers %d entry forms, want 2", got)
	}
	if !strings.Contains(body, `name="tournament_round" value="1"`) ||
		!strings.Contains(body, `name="tournament_round" value="2"`) {
		t.Error("the two legs do not carry different slots")
	}

	// The first leg only.
	rec := kioskPost(t, h, "/kiosk/tournaments/"+id+"/matches", cookie, url.Values{
		"home_id": {field[0].String()}, "away_id": {field[1].String()},
		"tournament_round": {"1"},
		"set_home_1":       {"11"}, "set_away_1": {"5"},
		"set_home_2": {"11"}, "set_away_2": {"7"},
	})
	if loc := rec.Header().Get("Location"); strings.Contains(loc, "fehler=") {
		t.Fatalf("the first leg was refused: %s", loc)
	}

	body = drawBody(t, h, cookie, id)
	if got := strings.Count(body, `name="tournament_round"`); got != 1 {
		t.Errorf("after one leg there are %d forms, want 1 — the return leg", got)
	}
	if !strings.Contains(body, `name="tournament_round" value="2"`) {
		t.Error("the return leg lost its form when the first leg was entered")
	}
}

// The form is the one part a caller can edit, so a slot it names has to exist.
func TestAResultCannotClaimASlotItDoesNotFill(t *testing.T) {
	h, store := kioskHandler(t)
	cookie := unlock(t, h)
	field := seedField(t, store)
	id := seedFormat(t, store, domain.TournamentRoundRobin, false, field)

	for _, round := range []string{"", "0", "2", "99"} {
		rec := kioskPost(t, h, "/kiosk/tournaments/"+id+"/matches", cookie, url.Values{
			"home_id": {field[0].String()}, "away_id": {field[1].String()},
			"tournament_round": {round},
			"set_home_1":       {"11"}, "set_away_1": {"5"},
			"set_home_2": {"11"}, "set_away_2": {"7"},
		})
		if !strings.Contains(rec.Header().Get("Location"), "fehler=") {
			t.Errorf("round %q was accepted", round)
		}
	}
}

// The final's slot exists from the start and its names arrive with the table.
func TestTheFinalWaitsForTheGroup(t *testing.T) {
	h, store := kioskHandler(t)
	cookie := unlock(t, h)
	field := seedField(t, store)
	id := seedFormat(t, store, domain.TournamentRoundRobin, true, field)

	body := drawBody(t, h, cookie, id)
	if !strings.Contains(body, "Finale") {
		t.Fatal("the draw does not show a final")
	}
	if !strings.Contains(body, "Steht fest, sobald alle Gruppenspiele gewertet sind.") {
		t.Error("the final does not say why it has no names yet")
	}
	// Two of two: one group match, one final.
	if !strings.Contains(body, "von 2 Spielen gewertet") {
		t.Error("the total does not count the final")
	}

	rec := kioskPost(t, h, "/kiosk/tournaments/"+id+"/matches", cookie, url.Values{
		"home_id": {field[0].String()}, "away_id": {field[1].String()},
		"tournament_round": {"1"},
		"set_home_1":       {"11"}, "set_away_1": {"5"},
		"set_home_2": {"11"}, "set_away_2": {"7"},
	})
	if loc := rec.Header().Get("Location"); strings.Contains(loc, "fehler=") {
		t.Fatalf("the group match was refused: %s", loc)
	}

	body = drawBody(t, h, cookie, id)
	if strings.Contains(body, "Steht fest, sobald") {
		t.Error("the final still has no names although the group is played out")
	}
	if !strings.Contains(body, `name="tournament_round" value="2"`) {
		t.Error("the final offers no entry form")
	}
}

// Two players who did not reach the final cannot play it, and neither can the
// right two before the group is out.
func TestTheFinalRefusesThePairTheTableDidNotName(t *testing.T) {
	h, store := kioskHandler(t)
	cookie := unlock(t, h)
	field := seedField(t, store)
	cesar, err := store.Players().Create(t.Context(), "Cesar", domain.DefaultTTR)
	if err != nil {
		t.Fatalf("creating Cesar: %v", err)
	}
	all := append(append([]uuid.UUID{}, field...), cesar.ID)
	id := seedFormat(t, store, domain.TournamentRoundRobin, true, all)

	final := url.Values{
		"home_id": {all[0].String()}, "away_id": {all[1].String()},
		"tournament_round": {"4"},
		"set_home_1":       {"11"}, "set_away_1": {"5"},
		"set_home_2": {"11"}, "set_away_2": {"7"},
	}
	rec := kioskPost(t, h, "/kiosk/tournaments/"+id+"/matches", cookie, final)
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "fehler=") {
		t.Error("a final was played before the group produced one")
	}
}

// The sentence under the picker is computed, because the number depends on
// the names, the format and the final at once.
func TestTheSizeSentenceFollowsTheForm(t *testing.T) {
	h, store := kioskHandler(t)
	field := seedField(t, store)
	ids := url.Values{}
	for _, p := range field {
		ids.Add("player_id", p.String())
	}

	// The time is asserted alongside the count on purpose: templ drops an
	// expression that starts its own line without failing to compile, and the
	// first version of this sentence shipped with the wrong number in it.
	for _, tc := range []struct {
		name, format, final, want, hours string
	}{
		{"two, one leg", "round_robin", "", "1 Spiele", "eine halbe Stunde"},
		{"two, return leg", "double_round_robin", "", "2 Spiele", "eine halbe Stunde"},
		{"two, with a final", "round_robin", "1", "2 Spiele", "eine halbe Stunde"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			q := url.Values{}
			for k, v := range ids {
				q[k] = v
			}
			q.Set("format", tc.format)
			if tc.final != "" {
				q.Set("with_final", tc.final)
			}
			body := listBody(t, h, "/fragments/tournament-size?"+q.Encode())
			if !strings.Contains(body, tc.want) {
				t.Errorf("the sentence does not say %q: %s", tc.want, body)
			}
			if !strings.Contains(body, tc.hours) {
				t.Errorf("the sentence does not say %q: %s", tc.hours, body)
			}
		})
	}

	if body := listBody(t, h, "/fragments/tournament-size"); !strings.Contains(body, "Mindestens zwei") {
		t.Error("an empty field does not ask for names")
	}
}

func seedFormat(t *testing.T, store *memStore, format domain.TournamentFormat,
	withFinal bool, field []uuid.UUID,
) string {
	t.Helper()

	created, err := store.tournaments.Create(t.Context(), domain.Tournament{
		Name: "Format", Format: format, Status: domain.TournamentOpen,
		CreatedBy: field[0], BestOf: 3, PointsToWin: 11,
		WithFinal: withFinal, Players: field,
	})
	if err != nil {
		t.Fatalf("seeding the tournament: %v", err)
	}
	return created.ID.String()
}

// The entry form disappears once a slot holds a result, but the form is not
// the guard: two results in one round would both count in the table, and only
// one of them could ever be shown in the schedule.
func TestASlotHoldsOneResult(t *testing.T) {
	h, store := kioskHandler(t)
	cookie := unlock(t, h)
	field := seedField(t, store)
	id := seedFormat(t, store, domain.TournamentRoundRobin, false, field)

	entry := url.Values{
		"home_id": {field[0].String()}, "away_id": {field[1].String()},
		"tournament_round": {"1"},
		"set_home_1":       {"11"}, "set_away_1": {"5"},
		"set_home_2": {"11"}, "set_away_2": {"7"},
	}

	if loc := kioskPost(t, h, "/kiosk/tournaments/"+id+"/matches", cookie, entry).
		Header().Get("Location"); strings.Contains(loc, "fehler=") {
		t.Fatalf("the first result was refused: %s", loc)
	}
	if loc := kioskPost(t, h, "/kiosk/tournaments/"+id+"/matches", cookie, entry).
		Header().Get("Location"); !strings.Contains(loc, "fehler=") {
		t.Error("the same slot took a second result")
	}
	if n := len(store.matches.all()); n != 1 {
		t.Errorf("got %d matches, want 1", n)
	}
}
