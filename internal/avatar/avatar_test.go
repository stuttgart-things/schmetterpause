package avatar_test

import (
	"errors"
	"io/fs"
	"math"
	"math/rand/v2"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/stuttgart-things/schmetterpause/internal/avatar"
	"github.com/stuttgart-things/schmetterpause/internal/templates"
	"github.com/stuttgart-things/schmetterpause/web"
)

const partsDir = "static/img/sthings"

func kit(t *testing.T) *avatar.Kit {
	t.Helper()
	k, err := avatar.Load(web.Static, partsDir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return k
}

// TestBladeIndexMatchesPaddleClass holds the default blade to the colour a
// player has carried since #64. If the two drift apart, everybody who has
// not chosen changes colour on deploy, and zaehlwerk, which copies the same
// function, disagrees with this page.
func TestBladeIndexMatchesPaddleClass(t *testing.T) {
	for range 500 {
		id := uuid.NewString()
		want := templates.PaddleClass(id)
		if got := "paddle-" + strconv.Itoa(avatar.BladeIndex(id)); got != want {
			t.Fatalf("BladeIndex(%s) gives %s, PaddleClass gives %s", id, got, want)
		}
	}
}

// TestBladesMatchTheStylesheet holds Blades to .paddle-0 … .paddle-6 in
// app.css, value and order. The index is shared with PaddleClass and with
// zaehlwerk; a reorder on one side would dress players differently on the
// two screens.
func TestBladesMatchTheStylesheet(t *testing.T) {
	css, err := fs.ReadFile(web.Static, "static/css/app.css")
	if err != nil {
		t.Fatal(err)
	}
	rule := regexp.MustCompile(`\.paddle-(\d+) \{ --paddle: (#[0-9a-f]{6}); \}`)
	found := rule.FindAllStringSubmatch(string(css), -1)
	if len(found) != len(avatar.Blades) {
		t.Fatalf("app.css has %d blade rules, Blades has %d entries", len(found), len(avatar.Blades))
	}
	for _, m := range found {
		i, _ := strconv.Atoi(m[1])
		if got := avatar.Blades[i].Hex; got != m[2] {
			t.Errorf("Blades[%d] = %s, app.css .paddle-%d = %s", i, got, i, m[2])
		}
	}
}

func TestDefaultIsTheDrawing(t *testing.T) {
	a := avatar.Default(uuid.NewString())
	if a.Fur != "orange" || a.Headdress != avatar.SameAsFur || a.Handle != avatar.SameAsFur ||
		a.Bling != avatar.NoBling || a.Blade == "" {
		t.Errorf("Default = %+v, want the drawing with a derived blade", a)
	}
	if err := avatar.Validate(a); err != nil {
		t.Errorf("Validate(Default) = %v", err)
	}
}

// TestDefaultForNobodyKeepsTheRedBlade: no player, no colour, as PaddleClass
// has it.
func TestDefaultForNobodyKeepsTheRedBlade(t *testing.T) {
	if got := avatar.Default("").Blade; got != "" {
		t.Errorf("Default(\"\").Blade = %q, want empty", got)
	}
}

// TestFillFallsBackPerPart: a stored key that no longer exists costs that
// one part, not the whole figure (docs/adr/0029).
func TestFillFallsBackPerPart(t *testing.T) {
	id := uuid.NewString()
	got := avatar.Fill(avatar.Avatar{Fur: "white", Headdress: "plaid", Handle: "red", Blade: "pink", Bling: "crown"}, id)
	want := avatar.Avatar{Fur: "white", Headdress: avatar.SameAsFur, Handle: "red", Blade: "pink", Bling: avatar.NoBling}
	if got != want {
		t.Errorf("Fill = %+v, want %+v", got, want)
	}
}

func TestValidateRejectsAnUnknownKey(t *testing.T) {
	a := avatar.Default(uuid.NewString())
	a.Fur = "grey"
	if err := avatar.Validate(a); !errors.Is(err, avatar.ErrUnknown) {
		t.Errorf("Validate(fur grey) = %v, want ErrUnknown", err)
	}
	a = avatar.Default(uuid.NewString())
	a.Blade = ""
	if err := avatar.Validate(a); err == nil {
		t.Error("Validate accepted an empty blade; a player who chooses chooses one of the seven")
	}
}

// TestRandomIsRepeatableAndReachesEverything: the demo field relies on the
// first, the point of rolling on the second.
func TestRandomIsRepeatableAndReachesEverything(t *testing.T) {
	roll := func(seed uint64) []avatar.Avatar {
		r := rand.New(rand.NewPCG(seed, seed))
		out := make([]avatar.Avatar, 3000)
		for i := range out {
			out[i] = avatar.Random(r)
		}
		return out
	}
	a, b := roll(7), roll(7)
	seen := map[string]bool{}
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("roll %d differs between two runs with the same seed", i)
		}
		if err := avatar.Validate(a[i]); err != nil {
			t.Fatalf("Random produced an invalid figure: %v", err)
		}
		for _, k := range []string{"fur:" + a[i].Fur, "head:" + a[i].Headdress, "handle:" + a[i].Handle,
			"blade:" + a[i].Blade, "bling:" + a[i].Bling} {
			seen[k] = true
		}
	}
	want := len(avatar.Furs) + len(avatar.Headdresses) + len(avatar.Handles) +
		len(avatar.Blades) + len(avatar.Blings)
	if len(seen) != want {
		t.Errorf("3000 rolls reached %d choices, want all %d", len(seen), want)
	}
}

func TestRenderTheDefault(t *testing.T) {
	k := kit(t)
	id := uuid.NewString()
	svg := k.Render(avatar.Default(id))

	// The blade keeps its fallback, which the pipeline's smoke test looks
	// for on every page that carries the mascot.
	if !strings.Contains(svg, "var(--paddle, #c82828)") {
		t.Error("the blade lost var(--paddle, #c82828)")
	}
	blade := avatar.Blades[avatar.BladeIndex(id)].Hex
	if !strings.Contains(svg, "--paddle:"+blade) {
		t.Errorf("the derived blade %s is not set", blade)
	}
	if n := strings.Count(svg, `class="sthings-face"`); n != 1 || !strings.Contains(svg, `data-face="neutral"`) {
		t.Errorf("%d faces in one figure, want the neutral one only", n)
	}
	if strings.Contains(svg, "sthings-bling") {
		t.Error("the default wears bling")
	}
	if strings.Count(svg, "<svg") != 1 {
		t.Error("the parts' own <svg> elements leaked into the figure")
	}
}

func TestRenderEveryBling(t *testing.T) {
	k := kit(t)
	for _, b := range avatar.Blings {
		svg := k.Render(avatar.Avatar{Bling: b.Key})
		if b.Key != avatar.NoBling && !strings.Contains(svg, `data-bling="`+b.Key+`"`) {
			t.Errorf("bling %q missing", b.Key)
		}
	}
}

// TestRenderLiveCarriesEveryFace: the figure next to a running match switches
// its face by attribute alone, so every face has to be in it, with the
// requested one picked.
func TestRenderLiveCarriesEveryFace(t *testing.T) {
	k := kit(t)
	svg := k.RenderLive(avatar.Default(uuid.NewString()), "surprised")
	if !strings.Contains(svg, `class="sthings" data-face="surprised"`) {
		t.Errorf("the requested face is not picked: %s", svg[:160])
	}
	for _, f := range avatar.Faces {
		if !strings.Contains(svg, `<g class="sthings-face" data-face="`+f.Key+`"`) {
			t.Errorf("face %q missing", f.Key)
		}
		if !strings.Contains(svg, `.sthings[data-face="`+f.Key+`"] .sthings-face[data-face="`+f.Key+`"]{display:inline}`) {
			t.Errorf("no rule shows face %q", f.Key)
		}
	}
	if got := k.RenderLive(avatar.Avatar{}, `x"><script>`); !strings.Contains(got, `data-face="neutral"`) || strings.Contains(got, "<script>") {
		t.Error("an unknown face does not fall back to neutral")
	}
}

// TestRenderTakesNothingButPaletteValues: whatever arrives as a key, the
// style attribute only ever holds hex values from the palettes.
func TestRenderTakesNothingButPaletteValues(t *testing.T) {
	k := kit(t)
	svg := k.Render(avatar.Avatar{Fur: `#000;background:url(x)`, Blade: `"><script>`})
	if strings.Contains(svg, "url(x)") || strings.Contains(svg, "<script>") {
		t.Fatalf("input reached the markup: %s", svg[:200])
	}
	style := regexp.MustCompile(`style="([^"]*)"`).FindStringSubmatch(svg)[1]
	for _, decl := range strings.Split(style, ";") {
		if !regexp.MustCompile(`^--[a-z-]+:#[0-9a-f]{6}$`).MatchString(decl) {
			t.Errorf("style declaration %q is not a palette value", decl)
		}
	}
}

// TestPartsFollowTheBriefing: docs/maskottchen-briefing.md rules out styles,
// strokes, transforms and clip paths in the drawing, because the page and not
// the file decides how it looks, and a stroke scales differently from the
// rest at 28 px. The v5 package had all four; this keeps them from coming
// back with the next part somebody draws. The GIF for the LED panel depends
// on it too: its rasteriser draws filled paths and nothing else
// (docs/adr/0030).
func TestPartsFollowTheBriefing(t *testing.T) {
	entries, err := fs.ReadDir(web.Static, partsDir)
	if err != nil {
		t.Fatal(err)
	}
	forbidden := []string{"<style", "style=", "stroke", "transform", "clip-path", "<mask", "<image", "<text", "Gradient", "filter"}
	for _, e := range entries {
		b, err := fs.ReadFile(web.Static, partsDir+"/"+e.Name())
		if err != nil {
			t.Fatal(err)
		}
		for _, f := range forbidden {
			if strings.Contains(string(b), f) {
				t.Errorf("%s contains %q", e.Name(), f)
			}
		}
		if !strings.Contains(string(b), `viewBox="0 0 420 420"`) {
			t.Errorf("%s is not on the 420 viewBox", e.Name())
		}
	}
}

// TestPalettesKeepTheirDistances holds the palettes to the measurements in
// docs/adr/0029, so a colour added later is measured rather than eyeballed.
// dE76 in CIELAB, the measure the blade colours in app.css were picked by.
func TestPalettesKeepTheirDistances(t *testing.T) {
	const outline, background = "#080808", "#0a0a12"
	for _, f := range avatar.Furs {
		for _, b := range avatar.Blades {
			if d := deltaE(f.Hex, b.Hex); d < 38 {
				t.Errorf("fur %s is %.1f from blade %s, want at least 38", f.Key, d, b.Key)
			}
		}
		if d := deltaE(f.Hex, background); d < 56 {
			t.Errorf("fur %s is %.1f from the page, want at least 56", f.Key, d)
		}
	}
	for _, h := range avatar.Headdresses {
		if h.Hex != "" && deltaE(h.Hex, outline) < 74 {
			t.Errorf("headdress %s is %.1f from the outline, want at least 74", h.Key, deltaE(h.Hex, outline))
		}
	}
	for _, h := range avatar.Handles {
		if h.Hex != "" && deltaE(h.Hex, outline) < 58 {
			t.Errorf("handle %s is %.1f from the outline, want at least 58", h.Key, deltaE(h.Hex, outline))
		}
	}
}

func deltaE(a, b string) float64 {
	l1, a1, b1 := lab(a)
	l2, a2, b2 := lab(b)
	return math.Sqrt((l1-l2)*(l1-l2) + (a1-a2)*(a1-a2) + (b1-b2)*(b1-b2))
}

// lab converts #rrggbb to CIELAB under D65.
func lab(hex string) (l, a, b float64) {
	v, _ := strconv.ParseUint(hex[1:], 16, 32)
	lin := func(c uint64) float64 {
		s := float64(c) / 255
		if s > 0.04045 {
			return math.Pow((s+0.055)/1.055, 2.4)
		}
		return s / 12.92
	}
	r, g, bl := lin(v>>16&0xff), lin(v>>8&0xff), lin(v&0xff)
	x := (0.4124*r + 0.3576*g + 0.1805*bl) / 0.95047
	y := 0.2126*r + 0.7152*g + 0.0722*bl
	z := (0.0193*r + 0.1192*g + 0.9505*bl) / 1.08883
	f := func(t float64) float64 {
		if t > 0.008856 {
			return math.Cbrt(t)
		}
		return 7.787*t + 16.0/116
	}
	return 116*f(y) - 16, 500 * (f(x) - f(y)), 200 * (f(y) - f(z))
}
