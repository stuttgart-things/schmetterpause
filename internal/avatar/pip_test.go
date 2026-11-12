package avatar

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/gif"
	"io/fs"
	"math/rand/v2"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/stuttgart-things/schmetterpause/web"
)

func TestPipPaletteFallbackAndRandom(t *testing.T) {
	var keys []string
	for _, c := range FurPalette("pip") {
		keys = append(keys, c.Key)
	}
	if !reflect.DeepEqual(keys, []string{"grey", "blue-grey"}) {
		t.Fatal(keys)
	}
	for _, key := range []string{"", "grey", "blue-grey"} {
		a := DefaultForCharacter("pip", "anna")
		a.Fur = key
		if err := Validate(a); err != nil {
			t.Fatal(err)
		}
		if got := Fill(a, "anna"); got != a {
			t.Fatalf("lost Pip choice: %+v", got)
		}
	}

	for _, key := range []string{"sand", "chocolate", "cream"} {
		a := DefaultForCharacter("pip", "anna")
		a.Fur = key
		a.Headdress = "lapis"
		a.Bling = "ball"
		if !errors.Is(Validate(a), ErrUnknown) {
			t.Fatal("accepted unavailable options")
		}
		got := Fill(a, "anna")
		if got.Character != "pip" || got.Fur != "" || got.Headdress != SameAsFur || got.Bling != NoBling {
			t.Fatalf("fallback: %+v", got)
		}
	}
	if !reflect.DeepEqual(BlingPalette("pip"), []Option{{NoBling, "Kein Schmuck"}, {"initial", "Initiale"}}) {
		t.Fatal("unexpected jewellery")
	}
	rng := rand.New(rand.NewPCG(42, 21))
	seen := 0
	for range 1000 {
		a := Random(rng)
		if a.Character == "pip" {
			seen++
			if err := Validate(a); err != nil {
				t.Fatal(err)
			}
			if !hasOption(HankBlings, a.Bling) {
				t.Fatal(a)
			}
		}
	}
	if seen == 0 {
		t.Fatal("random never selected Pip")
	}
}

func TestPipAssetsUseFilledPathsOnly(t *testing.T) {
	forbidden := regexp.MustCompile(`(?i)\b(?:stroke(?:-[a-z]+)?|transform|mask|clip-path|filter)\s*=|<(?:\w+:)?(?:mask|clipPath|filter|image|text)\b`)
	files := []string{"base.svg", "bling-initial.svg"}
	for _, face := range Faces {
		files = append(files, "face-"+face.Key+".svg")
	}
	for _, file := range files {
		data, err := fs.ReadFile(web.Static, "static/img/pip/"+file)
		if err != nil {
			t.Fatal(err)
		}
		if forbidden.Match(data) {
			t.Fatalf("unsupported SVG in %s", file)
		}
	}
}

func TestPipHeadbandPaletteAndRendering(t *testing.T) {
	c, err := LoadCollection(web.Static, "static/img")
	if err != nil {
		t.Fatal(err)
	}
	for _, fur := range PipFurs {
		a := DefaultForCharacter("pip", "anna")
		a.Fur = fur.Key
		if !strings.Contains(c.RenderFlat(a, "neutral"), `fill="`+fur.Hex+`"`) {
			t.Fatalf("missing Pip %s surface", fur.Key)
		}
	}
	for _, head := range HeaddressPalette("pip") {
		if head.Key == "lapis" {
			t.Fatal("Pip still offers Lapis")
		}
	}
	a := DefaultForCharacter("pip", "anna")
	a.Headdress = "lapis"
	if !errors.Is(Validate(a), ErrUnknown) {
		t.Fatal("Pip accepts Lapis")
	}
	if Fill(a, "anna").Headdress != SameAsFur {
		t.Fatal("Lapis did not fall back")
	}
	a.Character = "vex"
	if err := Validate(a); err != nil {
		t.Fatal("changed blecki palette", err)
	}
}

func TestPipBeaksBelongToFacesWithinTwentyFivePercent(t *testing.T) {
	base, err := fs.ReadFile(web.Static, "static/img/pip/base.svg")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(base), "#f59a08") || strings.Contains(string(base), `data-pip-part="beak"`) {
		t.Fatal("beak in base")
	}
	seen := map[string]bool{}
	for _, face := range Faces {
		data, err := fs.ReadFile(web.Static, "static/img/pip/face-"+face.Key+".svg")
		if err != nil {
			t.Fatal(err)
		}
		match := regexp.MustCompile(`(?s)<g data-pip-part="beak">(.*?)</g>`).FindSubmatch(data)
		if match == nil {
			t.Fatal("missing beak", face.Key)
		}
		if seen[string(match[1])] {
			t.Fatal("duplicate beak", face.Key)
		}
		seen[string(match[1])] = true
		beak := regexp.MustCompile(`fill="[^"]+"`).ReplaceAllString(string(match[1]), `fill="#ffffff"`)
		img, err := rasterise(`<svg viewBox="0 0 420 420">`+beak+`</svg>`, 420)
		if err != nil {
			t.Fatal(err)
		}
		minX, minY, maxX, maxY := 420, 420, 0, 0
		for y := 0; y < 420; y++ {
			for x := 0; x < 420; x++ {
				if img.RGBAAt(x, y).R >= 128 {
					minX = min(minX, x)
					minY = min(minY, y)
					maxX = max(maxX, x)
					maxY = max(maxY, y)
				}
			}
		}
		w, h := maxX-minX+1, maxY-minY+1
		if w < 45 || w > 77 || h < 36 || h > 62 {
			t.Errorf("%s beak %dx%d exceeds native size tolerance", face.Key, w, h)
		}
	}
}

func TestPipExpressionGroupsRemainVisibleAt28Pixels(t *testing.T) {
	collection, err := LoadCollection(web.Static, "static/img")
	if err != nil {
		t.Fatal(err)
	}
	a := DefaultForCharacter("pip", "")
	images := map[string]*image.RGBA{}
	for _, face := range Faces {
		images[face.Key], err = rasterise(collection.RenderFlat(a, face.Key), 28)
		if err != nil {
			t.Fatal(err)
		}
	}
	// Positive, surprised and negative must remain distinguishable even when
	// reduced to coarse luminance rather than relying on antialiasing speckles.
	for _, pair := range [][2]string{{"neutral", "surprised"}, {"happy", "surprised"}, {"neutral", "angry"}, {"happy", "sad"}, {"surprised", "angry"}, {"surprised", "sad"}} {
		changed := 0
		for y := 0; y < 28; y++ {
			for x := 0; x < 28; x++ {
				one := color.GrayModel.Convert(images[pair[0]].At(x, y)).(color.Gray).Y / 32
				two := color.GrayModel.Convert(images[pair[1]].At(x, y)).(color.Gray).Y / 32
				if one != two {
					changed++
				}
			}
		}
		if changed < 3 {
			t.Errorf("%s/%s differ at only %d quantised pixels", pair[0], pair[1], changed)
		}
	}
	coatPixels := 0
	for y := 0; y < 28; y++ {
		for x := 0; x < 28; x++ {
			c := images["neutral"].RGBAAt(x, y)
			if int(c.R) >= 40 && int(c.R) <= 54 && int(c.G) >= 71 && int(c.G) <= 85 && int(c.B) >= 161 && int(c.B) <= 175 {
				coatPixels++
			}
		}
	}
	if coatPixels < 12 {
		t.Errorf("original coat appears in only %d pixels", coatPixels)
	}
}

func TestPipSilverRequiresANonGreyCoat(t *testing.T) {
	for _, fur := range []string{"", "grey", "blue-grey"} {
		a := DefaultForCharacter("pip", "anna")
		a.Fur = fur
		a.Headdress = "silver"
		allowed := fur != "grey"
		if got := has(HeaddressPalette("pip", fur), "silver"); got != allowed {
			t.Errorf("silver offered for %q: %v", fur, got)
		}
		if (Validate(a) == nil) != allowed {
			t.Errorf("validation for %q", fur)
		}
		filled := Fill(a, "anna")
		if allowed && filled.Headdress != "silver" {
			t.Fatal("legal silver removed")
		}
		if !allowed && filled.Headdress != SameAsFur {
			t.Fatal("grey/silver did not fall back to Original")
		}
		if HeaddressPalette("pip", fur)[0].Name != "Original" {
			t.Fatal("wrong Pip default headband label")
		}
	}
	rng := rand.New(rand.NewPCG(73, 99))
	seenGrey := false
	seenSilver := false
	for range 3000 {
		a := Random(rng)
		if err := Validate(a); err != nil {
			t.Fatal(err)
		}
		if a.Character != "pip" {
			continue
		}
		if a.Fur == "grey" {
			seenGrey = true
			if a.Headdress == "silver" {
				t.Fatal("random grey/silver")
			}
		}
		if a.Headdress == "silver" {
			seenSilver = true
		}
	}
	if !seenGrey || !seenSilver {
		t.Fatal("missing legal random options")
	}
}

func TestPip64PixelExpressionSeparation(t *testing.T) {
	c, err := LoadCollection(web.Static, "static/img")
	if err != nil {
		t.Fatal(err)
	}
	current := c.kits["pip"]
	a := DefaultForCharacter("pip", "")
	separation := func(k *Kit) int {
		t.Helper()
		images := map[string]image.Image{}
		for _, face := range Faces {
			data, err := k.GIF(a, []Frame{{Face: face.Key}}, 64)
			if err != nil {
				t.Fatal(err)
			}
			images[face.Key], err = gif.Decode(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
		}
		minimum := 64 * 64
		for i, one := range Faces {
			for _, two := range Faces[i+1:] {
				changed := 0
				for y := 0; y < 64; y++ {
					for x := 0; x < 64; x++ {
						l := color.GrayModel.Convert(images[one.Key].At(x, y)).(color.Gray).Y / 32
						r := color.GrayModel.Convert(images[two.Key].At(x, y)).(color.Gray).Y / 32
						if l != r {
							changed++
						}
					}
				}
				if changed < minimum {
					t.Logf("closest pair: %s/%s = %d", one.Key, two.Key, changed)
				}
				minimum = min(minimum, changed)
			}
		}
		return minimum
	}
	now := separation(current)
	t.Logf("minimum pair separation in 64px quantised GIFs: current=%d target=114", now)
	if now < 114 {
		t.Errorf("sthings-style expression separation regressed: %d < 114", now)
	}
}

func TestPipHasNoCheeks(t *testing.T) {
	files := []string{"base.svg"}
	for _, face := range Faces {
		files = append(files, "face-"+face.Key+".svg")
	}
	for _, name := range files {
		data, err := fs.ReadFile(web.Static, "static/img/pip/"+name)
		if err != nil {
			t.Fatal(err)
		}
		for _, forbidden := range []string{`data-pip-part="cheeks"`, "#f5b5ce", "#f294b8", "#ec6f74", "#f6d4e0"} {
			if strings.Contains(strings.ToLower(string(data)), forbidden) {
				t.Errorf("%s still contains %s", name, forbidden)
			}
		}
	}
}

func TestHankRacketPathsStayInsideCanvasMargin(t *testing.T) {
	data, err := fs.ReadFile(web.Static, "static/img/pip/base.svg")
	if err != nil {
		t.Fatal(err)
	}
	group := regexp.MustCompile(`(?s)<g data-pip-part="racket">(.*?)</g>\s*<g data-pip-part="flipper">`).FindSubmatch(data)
	if group == nil {
		t.Fatal("missing separate racket/flipper groups")
	}
	paths := regexp.MustCompile(`<path\b[^>]*/>`).FindAllString(string(group[1]), -1)
	if len(paths) != 6 {
		t.Fatalf("unexpected racket path count: %d", len(paths))
	}
	for index, path := range paths {
		path = regexp.MustCompile(`fill="[^"]+"`).ReplaceAllString(path, `fill="#ffffff"`)
		// Overscan the SVG so paths beyond the 420 canvas cannot hide in clipping.
		img, err := rasterise(`<svg viewBox="-20 -20 460 460">`+path+`</svg>`, 460)
		if err != nil {
			t.Fatal(err)
		}
		painted := 0
		for y := 0; y < 460; y++ {
			for x := 0; x < 460; x++ {
				if img.RGBAAt(x, y).R <= 8 {
					continue
				}
				painted++
				if x < 28 || x >= 432 || y < 28 || y >= 432 {
					t.Fatalf("racket path %d violates 8-unit margin at %d,%d", index, x-20, y-20)
				}
			}
		}
		if painted == 0 {
			t.Fatalf("racket path %d is missing", index)
		}
	}
}

// The previous two-glint/no-sclera tests encoded the superseded eye design.
// The approved reference now uses sthings' combined white pupil cut-outs.
func TestPipEyesUseSthingsWhiteAndInkConstruction(t *testing.T) {
	for _, face := range Faces {
		data, err := fs.ReadFile(web.Static, "static/img/pip/face-"+face.Key+".svg")
		if err != nil {
			t.Fatal(err)
		}
		for _, side := range []string{"0", "1"} {
			match := regexp.MustCompile(`(?s)<g data-pip-part="eye-` + side + `">(.*?)</g>`).FindSubmatch(data)
			if match == nil {
				t.Fatalf("%s missing eye %s", face.Key, side)
			}
			if !bytes.Contains(match[1], []byte(`fill="#080808"`)) {
				t.Fatal("missing strong ink contour")
			}
			if face.Key != "happy" && !bytes.Contains(match[1], []byte(`fill="#f8f8f8"`)) {
				t.Fatal("missing white eye and glint")
			}
			if bytes.Contains(match[1], []byte("#14141f")) {
				t.Fatal("obsolete sclera-free oval")
			}
		}
	}
}
