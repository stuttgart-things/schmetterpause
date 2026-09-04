package server_test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/stuttgart-things/schmetterpause/internal/server"
)

// mustParseUUID turns the id reportedByAnna hands back into the type the
// fake stores it under.
func mustParseUUID(t *testing.T, s string) uuid.UUID {
	t.Helper()

	id, err := uuid.Parse(s)
	if err != nil {
		t.Fatalf("parsing the match id %q: %v", s, err)
	}
	return id
}

// The heading is the whole feature: before this, the person who typed a
// result was the only participant never told that it had not landed.
const waitingHeading = "Wartet auf den Gegner"

func TestTheReporterSeesTheirOwnWaitingResult(t *testing.T) {
	h, store, anna, _ := twoBrowsers(t)
	reportedByAnna(t, h, store, anna)

	body := fragment(t, h, "/fragments/pending", anna).Body.String()

	if !strings.Contains(body, waitingHeading) {
		t.Errorf("Anna is not told her result is still waiting: %s", body)
	}
	if !strings.Contains(body, "Bodo") {
		t.Errorf("the entry does not say who it is waiting on: %s", body)
	}
	// She typed 2:0 for herself, and it has to read that way round here too
	// — this list answers "did that go through", and a score she does not
	// recognise makes it fail at that.
	if !strings.Contains(body, "2:0 für dich") {
		t.Errorf("the result is not shown from Anna's side: %s", body)
	}
}

// TestTheReporterIsGivenNothingToPress is the rule this list must not soften.
// You may not confirm your own report; that step is the application's real
// defence against a joke score, and a button here would be the one place to
// get around it.
func TestTheReporterIsGivenNothingToPress(t *testing.T) {
	h, store, anna, _ := twoBrowsers(t)
	id := reportedByAnna(t, h, store, anna)

	body := fragment(t, h, "/fragments/pending", anna).Body.String()

	for _, action := range []string{"/confirm", "/dispute", "/correct"} {
		if strings.Contains(body, "/matches/"+id+action) {
			t.Errorf("Anna is offered %s on her own report: %s", action, body)
		}
	}
	// Nor the heading that comes with work to do.
	if strings.Contains(body, "Zu bestätigen") {
		t.Errorf("Anna is told she has something to confirm: %s", body)
	}
}

func TestAConfirmedResultLeavesTheWaitingList(t *testing.T) {
	h, store, anna, bodo := twoBrowsers(t)
	id := reportedByAnna(t, h, store, anna)

	if rec := post(t, h, "/matches/"+id+"/confirm", bodo); rec.Code != http.StatusOK {
		t.Fatalf("Bodo confirming: status %d: %s", rec.Code, rec.Body.String())
	}

	body := fragment(t, h, "/fragments/pending", anna).Body.String()
	if strings.Contains(body, waitingHeading) {
		t.Errorf("the settled result is still listed as waiting: %s", body)
	}
}

// TestADisputedResultIsListedOnceNotTwice pins the one overlap between the
// two queries. A contested match is in PendingFor for BOTH players, because
// either may put it right — so if the waiting query also claimed it, its
// reporter would see the same match under two headings and read it as two.
func TestADisputedResultIsListedOnceNotTwice(t *testing.T) {
	h, store, anna, bodo := twoBrowsers(t)
	id := reportedByAnna(t, h, store, anna)

	if rec := post(t, h, "/matches/"+id+"/dispute", bodo); rec.Code != http.StatusOK {
		t.Fatalf("Bodo disputing: status %d: %s", rec.Code, rec.Body.String())
	}

	body := fragment(t, h, "/fragments/pending", anna).Body.String()

	if strings.Contains(body, waitingHeading) {
		t.Errorf("the contested match is listed as merely waiting too: %s", body)
	}
	// It belongs in the other list, where she can correct it.
	if !strings.Contains(body, "/matches/"+id+"/correct") {
		t.Errorf("Anna is not offered the correction she can make: %s", body)
	}
}

// TestAFreshResultCarriesNoAge is the case that would ruin the field. The
// median confirmation in the measurement week was 4.8 minutes; putting "seit
// 2 Minuten" on that is how a reader learns to skip the label entirely.
func TestAFreshResultCarriesNoAge(t *testing.T) {
	h, store, anna, _ := twoBrowsers(t)
	reportedByAnna(t, h, store, anna)

	body := fragment(t, h, "/fragments/pending", anna).Body.String()
	if strings.Contains(body, `class="age`) {
		t.Errorf("a result typed seconds ago is given an age: %s", body)
	}
}

func TestAnOldResultIsMarkedOnBothSides(t *testing.T) {
	h, store, anna, bodo := twoBrowsers(t)
	id := reportedByAnna(t, h, store, anna)

	// 22 hours is one of the two that prompted issue #159.
	store.matches.backdate(mustParseUUID(t, id), time.Now().Add(-22*time.Hour))

	forAnna := fragment(t, h, "/fragments/pending", anna).Body.String()
	if !strings.Contains(forAnna, "seit 22 Stunden") {
		t.Errorf("the reporter is not told how long it has waited: %s", forAnna)
	}

	// The opponent is the one who can end the wait, so the age has to reach
	// them too — the badge in the top bar says how many, never which is old.
	forBodo := fragment(t, h, "/fragments/pending", bodo).Body.String()
	if !strings.Contains(forBodo, "seit 22 Stunden") {
		t.Errorf("the opponent is not told how long it has waited: %s", forBodo)
	}
}

func TestPastADayTheEntryIsMarkedStale(t *testing.T) {
	h, store, anna, _ := twoBrowsers(t)
	id := reportedByAnna(t, h, store, anna)
	store.matches.backdate(mustParseUUID(t, id), time.Now().Add(-3*24*time.Hour))

	body := fragment(t, h, "/fragments/pending", anna).Body.String()
	if !strings.Contains(body, "seit 3 Tagen") {
		t.Errorf("the age is not counted in days: %s", body)
	}
	if !strings.Contains(body, "stale") {
		t.Errorf("a three-day-old result is not marked: %s", body)
	}
}

// TestWaitedSince pins the edges. The buckets are the whole point of the
// helper, and every one of them is a boundary somebody will land on.
func TestWaitedSince(t *testing.T) {
	now := time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC)

	cases := []struct {
		name  string
		ago   time.Duration
		label string
		stale bool
	}{
		{"just typed", 30 * time.Second, "", false},
		{"the median confirmation", 5 * time.Minute, "", false},
		{"just under an hour", 59 * time.Minute, "", false},
		{"exactly an hour", time.Hour, "seit einer Stunde", false},
		{"ninety minutes still reads as one", 90 * time.Minute, "seit einer Stunde", false},
		{"two hours", 2 * time.Hour, "seit 2 Stunden", false},
		{"the Thursday specimens", 22 * time.Hour, "seit 22 Stunden", false},
		{"just under the line", 23*time.Hour + 59*time.Minute, "seit 23 Stunden", false},
		{"exactly a day", server.StaleAfter, "seit einem Tag", true},
		{"a day and a half", 36 * time.Hour, "seit einem Tag", true},
		{"two days", 48 * time.Hour, "seit 2 Tagen", true},
		{"a long weekend", 4 * 24 * time.Hour, "seit 4 Tagen", true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			label, stale := server.WaitedSince(now, now.Add(-c.ago))
			if label != c.label {
				t.Errorf("label = %q, want %q", label, c.label)
			}
			if stale != c.stale {
				t.Errorf("stale = %v, want %v", stale, c.stale)
			}
		})
	}
}

// TestWaitedSinceSurvivesAClockThatWentBackwards is not academic: PlayedAt
// can be typed by hand at the kiosk, and a machine whose clock is a minute
// ahead would otherwise produce a negative duration and a label to match.
func TestWaitedSinceSurvivesAClockThatWentBackwards(t *testing.T) {
	now := time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC)

	label, stale := server.WaitedSince(now, now.Add(time.Hour))
	if label != "" || stale {
		t.Errorf("a match in the future reads %q/%v, want no label at all", label, stale)
	}
}
