package server_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/stuttgart-things/schmetterpause/internal/checkin"
	"github.com/stuttgart-things/schmetterpause/internal/domain"
)

// checkedIn records a check-in for player at half, the way a resolve would.
func checkedIn(t *testing.T, store *memStore, player uuid.UUID, half domain.Half) domain.Checkin {
	t.Helper()

	c, err := store.Checkins().Create(t.Context(), domain.Checkin{
		PlayerID: player, Half: half, Method: domain.CheckinRFID, CreatedAt: time.Now(),
	})
	if err != nil {
		t.Fatalf("checking in: %v", err)
	}
	return c
}

func checkedInResult(home, away domain.Player, homeCheckin, awayCheckin uuid.UUID) map[string]any {
	return map[string]any{
		"home_id":         home.ID,
		"away_id":         away.ID,
		"home_checkin_id": homeCheckin,
		"away_checkin_id": awayCheckin,
		"sets":            [][2]int{{11, 9}, {11, 7}},
		"best_of":         3,
		"points_to_win":   11,
	}
}

func createdMatch(t *testing.T, body []byte) uuid.UUID {
	t.Helper()

	var created struct {
		MatchID uuid.UUID `json:"match_id"`
		Status  string    `json:"status"`
	}
	if err := json.Unmarshal(body, &created); err != nil {
		t.Fatalf("decoding the response: %v", err)
	}
	if created.Status != string(domain.MatchPending) {
		t.Errorf("status = %q, want pending", created.Status)
	}
	return created.MatchID
}

// The whole of docs/adr/0024 in one pass, through the routes the Zählwerk
// and the players actually use: two fobs linked and resolved at the two
// halves, a result citing the two check-ins and no operator, pending until
// one of the two players confirms it.
func TestACheckedInMatchReportsWithoutAnOperator(t *testing.T) {
	h, store, _ := fobServer(t)
	annaCookie, anna := joined(t, h, store, "Anna")
	beaCookie, bea := joined(t, h, store, "Bea")

	if rec := post(t, h, "/fobs/window", annaCookie); rec.Code != http.StatusOK {
		t.Fatalf("Anna opening the window = %d", rec.Code)
	}
	left := decodeResolved(t, resolveFob(t, h, testScoreboardToken, testUID, "left"))
	if rec := post(t, h, "/fobs/window", beaCookie); rec.Code != http.StatusOK {
		t.Fatalf("Bea opening the window = %d", rec.Code)
	}
	right := decodeResolved(t, resolveFob(t, h, testScoreboardToken, strangerUID, "right"))
	if left.PlayerID != anna || right.PlayerID != bea {
		t.Fatalf("resolved %s and %s, want Anna and Bea", left.PlayerID, right.PlayerID)
	}

	annaP, _ := store.Players().ByID(t.Context(), anna)
	beaP, _ := store.Players().ByID(t.Context(), bea)
	rec := apiPost(t, h, testScoreboardToken, checkedInResult(annaP, beaP, left.CheckinID, right.CheckinID))
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST /api/results = %d %s, want 201", rec.Code, rec.Body)
	}
	id := createdMatch(t, rec.Body.Bytes())

	stored, err := store.Matches().ByID(t.Context(), id)
	if err != nil {
		t.Fatalf("reading the match: %v", err)
	}
	if stored.EnteredVia != domain.EnteredViaCheckin {
		t.Errorf("entered_via = %q, want checkin", stored.EnteredVia)
	}
	if stored.ReportedBy != uuid.Nil {
		t.Errorf("reported_by = %s, want nobody", stored.ReportedBy)
	}
	if stored.HomeCheckinID == nil || *stored.HomeCheckinID != left.CheckinID ||
		stored.AwayCheckinID == nil || *stored.AwayCheckinID != right.CheckinID {
		t.Errorf("check-ins = %v / %v, want %s / %s",
			stored.HomeCheckinID, stored.AwayCheckinID, left.CheckinID, right.CheckinID)
	}

	// Both players see it waiting on them: there is no reporter to leave
	// out, and a null one must not hide it.
	for _, who := range []struct {
		name   string
		cookie *http.Cookie
	}{{"Anna", annaCookie}, {"Bea", beaCookie}} {
		body := fragment(t, h, "/fragments/pending", who.cookie).Body.String()
		if !strings.Contains(body, "Zählwerk") || !strings.Contains(body, "pending-"+id.String()) {
			t.Errorf("%s's pending list does not show the checked-in result:\n%s", who.name, body)
		}
	}
	// And the loser reads the score credited to their opponent, not to
	// a reporter who is not there.
	if body := fragment(t, h, "/fragments/pending", beaCookie).Body.String(); !strings.Contains(body, "für Anna") {
		t.Errorf("Bea's entry does not credit Anna:\n%s", body)
	}

	if rec := post(t, h, "/matches/"+id.String()+"/confirm", beaCookie); rec.Code != http.StatusOK {
		t.Fatalf("Bea confirming = %d %s", rec.Code, rec.Body)
	}
	if after, _ := store.Matches().ByID(t.Context(), id); after.Status != domain.MatchConfirmed {
		t.Errorf("after Bea confirmed: %q, want confirmed", after.Status)
	}
}

// A check-in carries one result. The likeliest second copy is the Zählwerk
// retrying after a lost answer, and 409 tells it the first one arrived.
func TestACheckinCarriesOneResult(t *testing.T) {
	h, store := scoreboardHandler(t)
	home, away, other := threePlayers(t, store)
	hc := checkedIn(t, store, home.ID, domain.HalfLeft)
	ac := checkedIn(t, store, away.ID, domain.HalfRight)

	if rec := apiPost(t, h, testScoreboardToken, checkedInResult(home, away, hc.ID, ac.ID)); rec.Code != http.StatusCreated {
		t.Fatalf("first result = %d %s", rec.Code, rec.Body)
	}
	if rec := apiPost(t, h, testScoreboardToken, checkedInResult(home, away, hc.ID, ac.ID)); rec.Code != http.StatusConflict {
		t.Errorf("the same check-ins again = %d %s, want 409", rec.Code, rec.Body)
	}

	// One used check-in is enough to refuse: home checks in afresh, the
	// opponent's is still the old one.
	fresh := checkedIn(t, store, home.ID, domain.HalfLeft)
	if rec := apiPost(t, h, testScoreboardToken, checkedInResult(home, away, fresh.ID, ac.ID)); rec.Code != http.StatusConflict {
		t.Errorf("one used check-in = %d %s, want 409", rec.Code, rec.Body)
	}

	// A fresh pair is a new match.
	oc := checkedIn(t, store, other.ID, domain.HalfRight)
	if rec := apiPost(t, h, testScoreboardToken, checkedInResult(home, other, fresh.ID, oc.ID)); rec.Code != http.StatusCreated {
		t.Errorf("a fresh pair = %d %s, want 201", rec.Code, rec.Body)
	}
	if n := len(store.matches.rows); n != 2 {
		t.Errorf("stored %d matches, want 2", n)
	}
}

func TestCheckedInResultsAreRefused(t *testing.T) {
	cases := []struct {
		name string
		// build returns the body to send, given two fresh check-ins at
		// different halves.
		build func(t *testing.T, store *memStore, home, away, third domain.Player, hc, ac domain.Checkin) map[string]any
		want  int
	}{
		{
			name: "only one check-in",
			build: func(_ *testing.T, _ *memStore, home, away, _ domain.Player, hc, _ domain.Checkin) map[string]any {
				b := checkedInResult(home, away, hc.ID, uuid.Nil)
				delete(b, "away_checkin_id")
				return b
			},
			want: http.StatusBadRequest,
		},
		{
			name: "the same check-in twice",
			build: func(_ *testing.T, _ *memStore, home, away, _ domain.Player, hc, _ domain.Checkin) map[string]any {
				return checkedInResult(home, away, hc.ID, hc.ID)
			},
			want: http.StatusBadRequest,
		},
		{
			// Two proofs: the sender has not decided which path it is on.
			name: "check-ins and an operator",
			build: func(_ *testing.T, _ *memStore, home, away, third domain.Player, hc, ac domain.Checkin) map[string]any {
				b := checkedInResult(home, away, hc.ID, ac.ID)
				b["operator_id"] = third.ID
				return b
			},
			want: http.StatusBadRequest,
		},
		{
			name: "an unknown check-in",
			build: func(_ *testing.T, _ *memStore, home, away, _ domain.Player, hc, _ domain.Checkin) map[string]any {
				return checkedInResult(home, away, hc.ID, uuid.New())
			},
			want: http.StatusUnprocessableEntity,
		},
		{
			// The Zählwerk cannot name two players of its choosing as
			// checked in: the check-in says whose it is.
			name: "somebody else's check-in",
			build: func(_ *testing.T, _ *memStore, home, _, third domain.Player, hc, ac domain.Checkin) map[string]any {
				return checkedInResult(home, third, hc.ID, ac.ID)
			},
			want: http.StatusUnprocessableEntity,
		},
		{
			name: "home and away swapped",
			build: func(_ *testing.T, _ *memStore, home, away, _ domain.Player, hc, ac domain.Checkin) map[string]any {
				return checkedInResult(away, home, hc.ID, ac.ID)
			},
			want: http.StatusUnprocessableEntity,
		},
		{
			name: "both at the same half",
			build: func(t *testing.T, store *memStore, home, away, _ domain.Player, hc, _ domain.Checkin) map[string]any {
				left := checkedIn(t, store, away.ID, domain.HalfLeft)
				return checkedInResult(home, away, hc.ID, left.ID)
			},
			want: http.StatusUnprocessableEntity,
		},
		{
			name: "an expired check-in",
			build: func(_ *testing.T, store *memStore, home, away, _ domain.Player, hc, ac domain.Checkin) map[string]any {
				store.checkins.backdate(ac.ID, time.Now().Add(-checkin.Validity-time.Minute))
				return checkedInResult(home, away, hc.ID, ac.ID)
			},
			want: http.StatusUnprocessableEntity,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h, store := scoreboardHandler(t)
			home, away, third := threePlayers(t, store)
			hc := checkedIn(t, store, home.ID, domain.HalfLeft)
			ac := checkedIn(t, store, away.ID, domain.HalfRight)

			rec := apiPost(t, h, testScoreboardToken, tc.build(t, store, home, away, third, hc, ac))
			if rec.Code != tc.want {
				t.Errorf("got %d, want %d (%s)", rec.Code, tc.want, rec.Body)
			}
			var e struct {
				Error string `json:"error"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &e); err != nil || e.Error == "" {
				t.Errorf("a refusal must be JSON with a reason, got %q", rec.Body)
			}
			if n := len(store.matches.rows); n != 0 {
				t.Errorf("a refused result stored %d matches", n)
			}
		})
	}
}
