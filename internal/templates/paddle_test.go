package templates_test

import (
	"strconv"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/stuttgart-things/schmetterpause/internal/templates"
	"github.com/stuttgart-things/schmetterpause/web"
)

func TestPaddleClassIsStableForAPlayer(t *testing.T) {
	id := "6f1e2c34-5a7b-4c8d-9e0f-112233445566"

	first := templates.PaddleClass(id)
	if first == "" {
		t.Fatal("a known player got no colour")
	}
	// The colour has to survive a restart and be the same on every device,
	// or it says nothing about who somebody is.
	for range 20 {
		if got := templates.PaddleClass(id); got != first {
			t.Fatalf("PaddleClass() = %q, then %q — not stable", first, got)
		}
	}
}

func TestPaddleClassIsEmptyForNobody(t *testing.T) {
	// No player, no colour: the blade keeps the red it is drawn in, and the
	// class attribute must not gain a stray entry.
	if got := templates.PaddleClass(""); got != "" {
		t.Errorf("PaddleClass(\"\") = %q, want empty", got)
	}
}

// TestPaddleClassUsesTheWholePalette guards the mapping against a hash that
// collapses: a colour nobody is ever given is a colour that does not exist.
func TestPaddleClassUsesTheWholePalette(t *testing.T) {
	seen := map[string]int{}
	for range 2000 {
		seen[templates.PaddleClass(uuid.NewString())]++
	}

	if len(seen) != 7 {
		t.Errorf("%d of 7 colours are ever handed out: %v", len(seen), seen)
	}
	// Even coverage matters less than full coverage, but a class taking half
	// the office would be a broken hash rather than luck.
	for class, n := range seen {
		if n < 2000/7/2 {
			t.Errorf("%s went to only %d of 2000 players", class, n)
		}
	}
}

// TestEveryColourClassExistsInTheStylesheet is the join between the two
// halves: the count lives in Go, the values live in CSS, and nothing forces
// them to agree except this.
func TestEveryColourClassExistsInTheStylesheet(t *testing.T) {
	css := readStylesheet(t)

	for i := range 7 {
		want := ".paddle-" + strconv.Itoa(i) + " {"
		if !strings.Contains(css, want) {
			t.Errorf("the stylesheet has no rule %q", want)
		}
	}
	// And no rule beyond them, which would be a colour Go never assigns.
	if strings.Contains(css, ".paddle-7 {") {
		t.Error("the stylesheet defines a colour the code never hands out")
	}
}

func readStylesheet(t *testing.T) string {
	t.Helper()

	b, err := web.Static.ReadFile("static/css/app.css")
	if err != nil {
		t.Fatalf("reading the stylesheet: %v", err)
	}
	return string(b)
}
