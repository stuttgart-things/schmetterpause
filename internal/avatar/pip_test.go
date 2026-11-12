package avatar

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/gif"
	"io/fs"
	"math/rand/v2"
	"os"
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
	if !reflect.DeepEqual(BlingPalette("pip"), []Option{{NoBling, "Kein Schmuck"}}) {
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
			if a.Bling != NoBling {
				t.Fatal(a)
			}
		}
	}
	if seen == 0 {
		t.Fatal("random never selected Pip")
	}
}

func TestPipAssetsUseFilledPathsOnly(t *testing.T) {
	forbidden := regexp.MustCompile(`(?i)\b(?:stroke(?:-[a-z]+)?|transform|mask|clip-path|filter)\s*=|<(?:\w+:)?(?:mask|clipPath|filter|image)\b`)
	files := []string{"base.svg"}
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

func TestPipRacketClearsTheHead(t *testing.T) {
	data, err := fs.ReadFile(web.Static, "static/img/pip/base.svg")
	if err != nil {
		t.Fatal(err)
	}
	part := func(name string) *image.RGBA {
		t.Helper()
		match := regexp.MustCompile(`(?s)<g data-pip-part="` + name + `">(.*?)</g>`).FindSubmatch(data)
		if match == nil {
			t.Fatalf("missing %s", name)
		}
		svg := `<svg viewBox="0 0 420 420">` + string(match[1]) + `</svg>`
		// Resolve the same variables as RenderFlat for this geometry-only check.
		svg = varRef.ReplaceAllStringFunc(svg, func(s string) string { return varRef.FindStringSubmatch(s)[2] })
		svg = regexp.MustCompile(`fill="[^"]+"`).ReplaceAllString(svg, `fill="#ffffff"`)
		img, err := rasterise(svg, 420)
		if err != nil {
			t.Fatal(err)
		}
		return img
	}
	body, blade := part("body"), part("blade")
	for y := 0; y < 420; y++ {
		for x := 0; x < 420; x++ {
			if blade.RGBAAt(x, y).R < 128 {
				continue
			}
			for dy := -12; dy <= 12; dy++ {
				for dx := -12; dx <= 12; dx++ {
					if dx*dx+dy*dy >= 144 || y+dy > 315 {
						continue
					}
					if body.RGBAAt(x+dx, y+dy).R >= 128 {
						t.Fatalf("blade/head gap below 12 at %d,%d", x, y)
					}
				}
			}
		}
	}
}

func TestPipBeaksBelongToFacesAndRemainWithinNativeSize(t *testing.T) {
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
		if w < 54 || w > 68 || h < 43 || h > 55 {
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

func TestPipEyesKeepTwoVisibleHighlights(t *testing.T) {
	c, err := LoadCollection(web.Static, "static/img")
	if err != nil {
		t.Fatal(err)
	}
	a := DefaultForCharacter("pip", "")
	for _, face := range Faces {
		if face.Key == "happy" {
			continue
		}
		img, err := rasterise(c.RenderFlat(a, face.Key), 420)
		if err != nil {
			t.Fatal(err)
		}
		for index, anchor := range [][2]int{{146, 221}, {260, 192}} {
			cx, cy := anchor[0], anchor[1]
			hy := cy - 5
			if face.Key == "angry" || face.Key == "sad" || (face.Key == "skeptical" && index == 1) {
				hy = cy + 15
			}
			// Check centres and four interior points so a lid cannot turn a glint
			// into a missing pixel or a clipped triangle.
			for _, glint := range [][3]int{{cx - 4, hy, 5}, {cx + 9, cy + 18, 1}} {
				for _, offset := range [][2]int{{0, 0}, {glint[2], 0}, {-glint[2], 0}, {0, glint[2]}, {0, -glint[2]}} {
					pixel := img.RGBAAt(glint[0]+offset[0], glint[1]+offset[1])
					if pixel.R < 240 || pixel.G < 240 || pixel.B < 240 {
						t.Errorf("%s eye %d glint covered at %v: %v", face.Key, index, offset, pixel)
					}
				}
			}
		}
	}
}

func TestPipOpenEyesHaveOnlyRoundWhiteGlints(t *testing.T) {
	for _, face := range Faces {
		data, err := fs.ReadFile(web.Static, "static/img/pip/face-"+face.Key+".svg")
		if err != nil {
			t.Fatal(err)
		}
		eyes := regexp.MustCompile(`(?s)<g data-pip-part="beak">.*?</g>`).ReplaceAllString(string(data), "")
		whites := regexp.MustCompile(`<path fill="#f8f8f8"[^>]*/>`).FindAllString(eyes, -1)
		if face.Key == "happy" {
			if len(whites) != 0 {
				t.Fatal("changed happy crescents")
			}
			continue
		}
		if len(whites) != 4 {
			t.Fatalf("%s has %d white paths, want two glints per eye", face.Key, len(whites))
		}
		big, small := 0, 0
		for _, path := range whites {
			img, err := rasterise(`<svg viewBox="0 0 420 420">`+path+`</svg>`, 420)
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
			switch {
			case w >= 16 && w <= 19 && h >= 16 && h <= 19:
				big++
			case w >= 7 && w <= 9 && h >= 7 && h <= 9:
				small++
			default:
				t.Errorf("%s white shape %dx%d is not a permitted round glint (possible sclera)", face.Key, w, h)
			}
			if img.RGBAAt(minX, minY).R >= 128 {
				t.Error("rectangular glint")
			}
		}
		if big != 2 || small != 2 {
			t.Errorf("%s glints: big=%d small=%d", face.Key, big, small)
		}
	}
}

func TestPip64PixelExpressionsRemainAsDistinctAsAddbf59(t *testing.T) {
	c, err := LoadCollection(web.Static, "static/img")
	if err != nil {
		t.Fatal(err)
	}
	current := c.kits["pip"]
	old := *current
	old.faces = map[string]string{}
	for _, face := range Faces {
		data, err := os.ReadFile("../../docs/avatar-check/pip/before-sclera-removal/face-" + face.Key + ".svg")
		if err != nil {
			t.Fatal(err)
		}
		old.faces[face.Key], err = inner(string(data))
		if err != nil {
			t.Fatal(err)
		}
	}
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
	previous, now := separation(&old), separation(current)
	t.Logf("minimum pair separation in 64px quantised GIFs: addbf59=%d current=%d", previous, now)
	if now < previous {
		t.Errorf("expression separation regressed: %d < %d", now, previous)
	}
}
