package avatar

import (
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/stuttgart-things/schmetterpause/web"
)

func TestBallColourFallbackPreservationAndDerivedShadows(t *testing.T) {
	c, err := LoadCollection(web.Static, "static/img")
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"", "white", "unknown", "<script>"} {
		a := DefaultForCharacter("vex", "anna")
		a.Bling = "ball"
		a.BallColor = key
		if Fill(a, "anna").BallColor != "" {
			t.Errorf("%q does not fall back to white", key)
		}
		if !strings.Contains(c.RenderFlat(a, "neutral"), `fill="#f8f8f8"`) {
			t.Error("white missing")
		}
	}
	for _, tone := range BallColors {
		a := DefaultForCharacter("vex", "anna")
		a.Bling = "ball"
		a.BallColor = tone.Key
		if Fill(a, "anna") != a {
			t.Errorf("valid colour lost: %q", tone.Key)
		}
		flat := c.RenderFlat(a, "neutral")
		if !strings.Contains(flat, `fill="`+tone.Hex+`"`) || !strings.Contains(flat, `fill="`+tone.Shade+`"`) {
			t.Errorf("ball colour/shadow missing: %q", tone.Key)
		}
		for _, bling := range []string{""} {
			a.Bling = bling
			plain := a
			plain.BallColor = ""
			if c.Render(a) != c.Render(plain) {
				t.Error("dormant ball colour changes another accessory")
			}
			if Fill(a, "anna").BallColor != tone.Key {
				t.Error("dormant colour is not preserved")
			}
		}
	}
	if BallTone("").Shade != "#d9d9d9" || BallTone("orange").Shade != "#df781f" || BallTone("yellow").Shade != "#dfc533" {
		t.Error("shadows must scale channels by 7/8")
	}
}

func TestBallColourRandomAndResetDefaults(t *testing.T) {
	rng := rand.New(rand.NewPCG(55, 77))
	seen := map[string]bool{}
	for range 3000 {
		a := Random(rng)
		if a.Character == "vex" && a.Bling == "ball" {
			seen[a.BallColor] = true
		} else if a.BallColor != "" {
			t.Error("ball colour rolled without a ball")
		}
	}
	if len(seen) != len(BallColors) {
		t.Errorf("missing rolled colours: %v", seen)
	}
	if Default("anna").BallColor != "" || DefaultForCharacter("vex", "anna").BallColor != "" {
		t.Error("reset/default must select white")
	}
}
