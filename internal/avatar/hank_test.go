package avatar

import (
	"bytes"
	"github.com/stuttgart-things/schmetterpause/web"
	"image"
	"image/color"
	"image/gif"
	"io/fs"
	"math/rand/v2"
	"regexp"
	"strings"
	"testing"
)

func TestHankInitialDerivation(t *testing.T) {
	for _, c := range []struct{ name, want string }{{"anna", "A"}, {"  ölaf", "Ö"}, {"Ümit", "Ü"}, {"3D-Max", ""}, {"😀Tom", ""}, {"", ""}, {"  Änne", "Ä"}, {"<script>", ""}} {
		if got := InitialForName(c.name); got != c.want {
			t.Errorf("%q => %q, want %q", c.name, got, c.want)
		}
	}
}

func TestHankEmblemColoursAndFallback(t *testing.T) {
	for _, fur := range []string{"", "grey", "blue-grey"} {
		for _, head := range []string{SameAsFur, "gold", "ruby", "ivory", "silver"} {
			a := DefaultForCharacter("pip", "anna")
			a.Fur = fur
			a.Headdress = head
			a.Bling = "initial"
			filled := Fill(a, "anna")
			want := "#d742c5"
			switch filled.Headdress {
			case "gold":
				want = "#fdc82d"
			case "ruby":
				want = "#d9343f"
			case "ivory", "silver":
				want = "#2f4ea8"
				if fur == "grey" {
					want = "#8793a5"
				}
				if fur == "blue-grey" {
					want = "#587e9b"
				}
			}
			if got := HankEmblemColor(a); got != want {
				t.Errorf("%s/%s got %s want %s", fur, head, got, want)
			}
		}
	}
	for _, key := range []string{"snowflake", "fish", "Schneeflocke", "Fisch", "unknown"} {
		a := DefaultForCharacter("pip", "anna")
		a.Bling = key
		if Validate(a) == nil {
			t.Fatal("unknown emblem accepted", key)
		}
		if Fill(a, "anna").Bling != NoBling {
			t.Fatal("unknown emblem has no fallback", key)
		}
	}
	r := rand.New(rand.NewPCG(818, 917))
	seen := map[string]bool{}
	for range 2000 {
		a := Random(r)
		if a.Character == "pip" {
			seen[a.Bling] = true
			if !hasOption(HankBlings, a.Bling) {
				t.Fatal(a)
			}
		}
	}
	if !seen[NoBling] || !seen["initial"] || len(seen) != 2 {
		t.Fatal("random missed Hank choices", seen)
	}
}

func TestHankEveryGlyphIsPathsAndRendersInAllPipelines(t *testing.T) {
	c, err := LoadCollection(web.Static, "static/img")
	if err != nil {
		t.Fatal(err)
	}
	a := DefaultForCharacter("pip", "")
	a.Blade = "light-blue"
	a.Bling = "initial"
	forbidden := regexp.MustCompile(`(?i)\b(?:stroke(?:-[a-z]+)?|transform|mask|clip-path|filter)\s*=|<(?:\w+:)?(?:mask|clipPath|filter|image|text)\b`)
	if len(hankInitialGlyphs) != 29 {
		t.Fatal("incomplete glyphs", len(hankInitialGlyphs))
	}
	for _, r := range "ABCDEFGHIJKLMNOPQRSTUVWXYZÄÖÜ" {
		name := string(r)
		ctx := RenderContext{DisplayName: name}
		if forbidden.MatchString(hankInitialGlyphs[name]) || !strings.Contains(hankInitialGlyphs[name], "<path ") {
			t.Fatal("invalid glyph", name)
		}
		flat := c.RenderFlat(a, "neutral", ctx)
		if forbidden.MatchString(flat) || strings.Contains(flat, "var(") {
			t.Fatal("invalid flat emblem SVG", name)
		}
		if !strings.Contains(flat, `data-initial="`+name+`"`) {
			t.Fatal("wrong glyph", name)
		}
		if strings.Index(flat, `data-bling="initial"`) > strings.Index(flat, `class="sthings-face"`) {
			t.Fatal("emblem is not before face")
		}
		data, err := c.GIF(a, []Frame{{Face: "neutral"}}, 64, ctx)
		if err != nil {
			t.Fatal(err)
		}
		img, err := gif.Decode(bytes.NewReader(data))
		if err != nil || img.Bounds().Dx() != 64 {
			t.Fatal("invalid glyph GIF", name, err)
		}
	}
	for _, ctx := range []RenderContext{{}, {DisplayName: "3D-Max"}, {DisplayName: "😀Tom"}} {
		if !strings.Contains(c.Render(a, ctx), `data-initial="fallback"`) {
			t.Fatal("missing racket fallback")
		}
	}
	base, _ := fs.ReadFile(web.Static, "static/img/pip/bling-initial.svg")
	if forbidden.Match(base) {
		t.Fatal("unsupported emblem asset")
	}
}

func TestHankEmblemKeepsExpressionSeparation(t *testing.T) {
	c, err := LoadCollection(web.Static, "static/img")
	if err != nil {
		t.Fatal(err)
	}
	a := DefaultForCharacter("pip", "")
	a.Bling = "initial"
	for _, name := range []string{"", "Anna", "Ümit"} {
		images := map[string]image.Image{}
		for _, face := range Faces {
			data, err := c.GIF(a, []Frame{{Face: face.Key}}, 64, RenderContext{DisplayName: name})
			if err != nil {
				t.Fatal(err)
			}
			images[face.Key], err = gif.Decode(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
		}
		minimum := 4096
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
				minimum = min(minimum, changed)
			}
		}
		t.Logf("Hank initial %q: minimum separation %d", name, minimum)
		if minimum < 114 {
			t.Errorf("emblem regresses separation below 114: %d", minimum)
		}
	}
}
