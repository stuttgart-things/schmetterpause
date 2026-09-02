package server_test

import (
	"strconv"
	"strings"
	"testing"

	"github.com/stuttgart-things/schmetterpause/internal/match"
	"github.com/stuttgart-things/schmetterpause/internal/templates"
)

// rulesSheet is the printed page of house rules.
func rulesSheet(t *testing.T) string {
	t.Helper()

	return get(t, newHandler(newMemStore()), "/rules").Body.String()
}

// TestTheRulesSheetNamesEveryHouseRule holds the sheet to the four rules it
// exists for. A sheet that quietly loses one is a rule that gets argued about
// at the table.
func TestTheRulesSheetNamesEveryHouseRule(t *testing.T) {
	body := rulesSheet(t)

	for _, want := range []string{
		"Aufschlag diagonal",
		"Aufschlagwechsel nach",
		"Zwei Punkte Abstand",
		"Netzroller",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the sheet does not carry %q: %s", want, body)
		}
	}
}

// TestTheRulesSheetAgreesWithTheEntryForm is the reason the sheet is rendered
// rather than a static file. Two of the four rules are statements about the
// target score, and a sheet on the wall that contradicts the form is worse
// than no sheet: the form can be corrected, the printout cannot.
func TestTheRulesSheetAgreesWithTheEntryForm(t *testing.T) {
	body := rulesSheet(t)

	serve := strconv.Itoa(templates.ServeEvery(match.PointsToEleven))
	if !strings.Contains(body, "Aufschlagwechsel nach "+serve+" Punkten") {
		t.Errorf("the sheet does not change service after %s points: %s", serve, body)
	}

	deuce := strconv.Itoa(templates.DeuceFrom(match.PointsToEleven))
	if !strings.Contains(body, deuce+":"+deuce) {
		t.Errorf("the sheet does not name %s:%s as the score a set runs on from: %s", deuce, deuce, body)
	}
	if !strings.Contains(body, "bis "+strconv.Itoa(match.PointsToEleven)) {
		t.Errorf("the sheet does not name the target score: %s", body)
	}
}

// TestTheRulesSheetIsReachable keeps it one click away rather than a URL
// somebody has to be told about — the same reason the QR sheet is in the
// navigation.
func TestTheRulesSheetIsReachable(t *testing.T) {
	body := get(t, newHandler(newMemStore()), "/").Body.String()

	if !strings.Contains(body, `href="/rules"`) {
		t.Errorf("nothing links to the rules sheet: %s", body)
	}
	// With the tools, not among the pages somebody opens between two
	// matches.
	tools := body[strings.Index(body, `class="mainnav-tools"`):]
	if i := strings.Index(tools, "</span>"); i >= 0 {
		tools = tools[:i]
	}
	if !strings.Contains(tools, `href="/rules"`) {
		t.Errorf("the rules sheet is not among the tools: %s", tools)
	}
}

// TestTheRulesSheetPrintsLikeTheQRSheet: both go on the same wall, so the
// page has to be built out of the same frame the print rules are written
// against.
func TestTheRulesSheetPrintsLikeTheQRSheet(t *testing.T) {
	body := rulesSheet(t)

	if !strings.Contains(body, `class="sheet"`) {
		t.Errorf("the rules page is not a printable sheet: %s", body)
	}
	// The way back is on screen only: on paper it is a line nobody can
	// click.
	if !strings.Contains(body, `class="sheet-back no-print"`) {
		t.Errorf("the sheet prints its own navigation: %s", body)
	}
	if !strings.Contains(body, `class="sheet-mascot `+templates.PaddleRules) {
		t.Errorf("the sheet does not carry the mascot in its own colour: %s", body)
	}
}
