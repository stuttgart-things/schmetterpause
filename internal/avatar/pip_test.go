package avatar

import (
	"errors"
	"io/fs"
	"math/rand/v2"
	"reflect"
	"regexp"
	"testing"

	"github.com/stuttgart-things/schmetterpause/web"
)

func TestPipPaletteFallbackAndRandom(t *testing.T) {
	var keys []string
	for _, c := range FurPalette("pip") {
		keys = append(keys, c.Key)
	}
	if !reflect.DeepEqual(keys, []string{"sand", "chocolate"}) {
		t.Fatal(keys)
	}
	for _, key := range []string{"", "sand", "chocolate"} {
		a := DefaultForCharacter("pip", "anna")
		a.Fur = key
		if err := Validate(a); err != nil {
			t.Fatal(err)
		}
		if got := Fill(a, "anna"); got != a {
			t.Fatalf("lost Pip choice: %+v", got)
		}
	}
	a := DefaultForCharacter("pip", "anna")
	a.Fur = "cream"
	a.Bling = "ball"
	if !errors.Is(Validate(a), ErrUnknown) {
		t.Fatal("accepted unavailable options")
	}
	got := Fill(a, "anna")
	if got.Character != "pip" || got.Fur != "" || got.Bling != NoBling {
		t.Fatalf("fallback: %+v", got)
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
