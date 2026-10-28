package avatar_test

import (
	"bytes"
	"fmt"
	"image/color"
	"image/gif"
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/stuttgart-things/schmetterpause/internal/avatar"
)

// TestRenderFlatNeedsNoCSS: the GIF's rasteriser resolves neither custom
// properties nor a <style> rule (docs/adr/0030). A var( left in the flat
// figure is a part drawn in no colour at all, a second face is two faces on
// top of each other.
func TestRenderFlatNeedsNoCSS(t *testing.T) {
	k := kit(t)
	a := avatar.Random(newRand(7))
	for _, f := range avatar.Faces {
		svg := k.RenderFlat(a, f.Key)
		for _, banned := range []string{"var(", "<style", "--fur"} {
			if strings.Contains(svg, banned) {
				t.Errorf("face %s: the flat figure still has %q", f.Key, banned)
			}
		}
		if n := strings.Count(svg, `class="sthings-face"`); n != 1 {
			t.Errorf("face %s: %d faces in the flat figure, want 1", f.Key, n)
		}
		if !strings.Contains(svg, `data-face="`+f.Key+`"`) {
			t.Errorf("face %s: the flat figure shows another face", f.Key)
		}
	}
}

// TestTheGIFIsTheFigureCheering: the loop the panel plays, every frame the
// size asked for, in the figure's own colours.
func TestTheGIFIsTheFigureCheering(t *testing.T) {
	k := kit(t)
	a := avatar.Random(newRand(7))
	b, err := k.GIF(a, avatar.Cheer, avatar.DefaultGIFSize)
	if err != nil {
		t.Fatal(err)
	}
	g, err := gif.DecodeAll(bytes.NewReader(b))
	if err != nil {
		t.Fatalf("not a GIF: %v", err)
	}
	if len(g.Image) != len(avatar.Cheer) {
		t.Fatalf("%d frames, want %d", len(g.Image), len(avatar.Cheer))
	}
	if g.LoopCount != 0 {
		t.Errorf("LoopCount = %d, want 0 (forever)", g.LoopCount)
	}
	for i, f := range avatar.Cheer {
		if g.Delay[i] != f.Delay {
			t.Errorf("frame %d stands %d, want %d", i, g.Delay[i], f.Delay)
		}
		if b := g.Image[i].Bounds(); b.Dx() != 64 || b.Dy() != 64 {
			t.Errorf("frame %d is %v, want 64x64", i, b)
		}
	}

	// The fur is the largest area of the figure: if it is not drawn in the
	// colour the player chose, the colours did not make it through.
	fur := furOf(t, a)
	found := false
	for _, c := range g.Image[0].Pix {
		if colourEq(g.Image[0].Palette[c], fur) {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("no pixel of the fur %v in the first frame", fur)
	}

	// Frames differ: the face moves.
	if bytes.Equal(g.Image[0].Pix, g.Image[1].Pix) {
		t.Error("the first two frames are the same picture")
	}
}

// TestTheGIFSizeIsHeld: below 32 pixels the face is gone, above 256 nobody
// needs it, and a public route should not draw whatever size it is asked.
func TestTheGIFSizeIsHeld(t *testing.T) {
	k := kit(t)
	for asked, want := range map[int]int{1: avatar.MinGIFSize, 48: 48, 100000: avatar.MaxGIFSize} {
		b, err := k.GIF(avatar.Avatar{}, []avatar.Frame{{Face: "happy"}}, asked)
		if err != nil {
			t.Fatal(err)
		}
		g, err := gif.Decode(bytes.NewReader(b))
		if err != nil {
			t.Fatal(err)
		}
		if got := g.Bounds().Dx(); got != want {
			t.Errorf("asked for %d, got %d, want %d", asked, got, want)
		}
	}
}

func furOf(t *testing.T, a avatar.Avatar) color.RGBA {
	t.Helper()
	for _, c := range avatar.Furs {
		if c.Key == a.Fur {
			var r, g, b uint8
			if _, err := fmt.Sscanf(c.Hex, "#%02x%02x%02x", &r, &g, &b); err != nil {
				t.Fatal(err)
			}
			return color.RGBA{R: r, G: g, B: b, A: 0xff}
		}
	}
	t.Fatalf("fur %q not in the palette", a.Fur)
	return color.RGBA{}
}

func colourEq(c color.Color, want color.RGBA) bool {
	r, g, b, _ := c.RGBA()
	return uint8(r>>8) == want.R && uint8(g>>8) == want.G && uint8(b>>8) == want.B
}

func newRand(seed uint64) *rand.Rand {
	return rand.New(rand.NewPCG(seed, seed))
}
