package avatar_test

import (
	"bytes"
	"errors"
	"image/gif"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/stuttgart-things/schmetterpause/internal/avatar"
	"github.com/stuttgart-things/schmetterpause/web"
)

func TestCharacterCollectionKeepsLegacyDrawing(t *testing.T) {
	c, err := avatar.LoadCollection(web.Static, "static/img")
	if err != nil {
		t.Fatal(err)
	}
	legacy := kit(t)
	for _, a := range []avatar.Avatar{avatar.Default("anna"), {Fur: "cream", Headdress: "lapis", Handle: "red", Blade: "pink", Bling: "chain"}} {
		if c.Render(a) != legacy.Render(a) {
			t.Error("the collection changed the existing mascot")
		}
		if c.RenderLive(a, "happy") != legacy.RenderLive(a, "happy") {
			t.Error("the collection changed existing live faces")
		}
	}
	a := avatar.Default("anna")
	a.Character = `unknown"><script>`
	if got := c.Render(a); got != legacy.Render(avatar.Default("anna")) {
		t.Error("unknown stored character did not fall back safely")
	}
}

func TestCharacterChoicesAndFallbacks(t *testing.T) {
	for _, ch := range avatar.Characters {
		a := avatar.DefaultForCharacter(ch.Key, "anna")
		if err := avatar.Validate(a); err != nil {
			t.Errorf("%s default: %v", ch.Name, err)
		}
		a.Blade = "pink"
		a.Handle = "blue"
		a.Headdress = "emerald"
		if got := avatar.Fill(a, "anna"); got != a {
			t.Errorf("%s lost valid choices: %+v", ch.Name, got)
		}
		a.Fur = "plaid"
		a.Bling = "chain"
		filled := avatar.Fill(a, "anna")
		if ch.Key != "" && (filled.Fur != "" || filled.Bling != "") {
			t.Errorf("%s did not restore original material / remove old jewellery", ch.Name)
		}
	}
	bad := avatar.Default("anna")
	bad.Character = "missing"
	if !errors.Is(avatar.Validate(bad), avatar.ErrUnknown) {
		t.Error("unknown submitted character accepted")
	}
	bad = avatar.DefaultForCharacter("vex", "anna")
	bad.Bling = "chain"
	if !errors.Is(avatar.Validate(bad), avatar.ErrUnknown) {
		t.Error("ill-fitting jewellery accepted")
	}
}

func TestEveryCharacterRendersLiveAndCheers(t *testing.T) {
	c, err := avatar.LoadCollection(web.Static, "static/img")
	if err != nil {
		t.Fatal(err)
	}
	for _, ch := range avatar.Characters {
		t.Run(ch.Name, func(t *testing.T) {
			a := avatar.DefaultForCharacter(ch.Key, "anna")
			svg := c.RenderLive(a, "surprised")
			if ch.Key != "" && !strings.Contains(svg, `data-character="`+ch.Key+`"`) {
				t.Error("wrong character")
			}
			if !strings.Contains(svg, `data-face="surprised"`) {
				t.Error("requested expression missing")
			}
			for _, f := range avatar.Faces {
				if strings.Count(svg, `<g class="sthings-face" data-face="`+f.Key+`"`) != 1 {
					t.Errorf("missing or duplicated %s", f.Key)
				}
			}
			if strings.Count(c.Render(a), `class="sthings-face"`) != 1 {
				t.Error("profile has more than its neutral face")
			}
			b, err := c.GIF(a, avatar.Cheer, 64)
			if err != nil {
				t.Fatal(err)
			}
			g, err := gif.DecodeAll(bytes.NewReader(b))
			if err != nil {
				t.Fatal(err)
			}
			if len(g.Image) != len(avatar.Cheer) {
				t.Fatalf("wrong cheer length %d", len(g.Image))
			}
			if bytes.Equal(g.Image[0].Pix, g.Image[1].Pix) {
				t.Error("happy and surprised are identical on the LED panel")
			}
		})
	}
}

func TestCollectionRejectsAnIncompleteCharacter(t *testing.T) {
	files := fstest.MapFS{}
	err := fs.WalkDir(web.Static, "static/img", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		b, err := fs.ReadFile(web.Static, p)
		if err != nil {
			return err
		}
		files[p] = &fstest.MapFile{Data: b}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	delete(files, "static/img/vex/face-happy.svg")
	if _, err := avatar.LoadCollection(files, "static/img"); err == nil || !strings.Contains(err.Error(), "blecki") {
		t.Errorf("missing blecki expression: %v", err)
	}
}

func TestRemovedCharactersCannotBeSelected(t *testing.T) {
	c, err := avatar.LoadCollection(web.Static, "static/img")
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"fynn", "chomp", "pogo", "kiki"} {
		t.Run(key, func(t *testing.T) {
			a := avatar.Avatar{Character: key, Headdress: avatar.SameAsFur, Handle: avatar.SameAsFur, Blade: "pink"}
			if !errors.Is(avatar.Validate(a), avatar.ErrUnknown) {
				t.Error("removed character can still be saved")
			}
			expected := avatar.Default("")
			expected.Blade = "pink"
			if got := avatar.Fill(a, ""); got != expected {
				t.Errorf("stored character fallback = %+v; want %+v", got, expected)
			}
			if c.Render(a) != c.Render(expected) {
				t.Error("stored removed character does not render as sthings")
			}
			if _, err := fs.Stat(web.Static, "static/img/"+key); !errors.Is(err, fs.ErrNotExist) {
				t.Errorf("removed assets still embedded: %v", err)
			}
		})
	}
}

func TestCharacterExpressionPreviewsContainOneFace(t *testing.T) {
	c, err := avatar.LoadCollection(web.Static, "static/img")
	if err != nil {
		t.Fatal(err)
	}
	for _, ch := range avatar.Characters {
		a := avatar.DefaultForCharacter(ch.Key, "anna")
		for _, f := range avatar.Faces {
			svg := c.RenderFace(a, f.Key)
			if strings.Count(svg, `class="sthings-face"`) != 1 || !strings.Contains(svg, `<g class="sthings-face" data-face="`+f.Key+`"`) {
				t.Errorf("%s/%s: preview does not isolate its expression", ch.Name, f.Key)
			}
			if strings.Contains(svg, "<style") {
				t.Errorf("%s/%s: static preview carries live switching styles", ch.Name, f.Key)
			}
		}
		if c.RenderFace(a, `bad"><script>`) != c.RenderFace(a, "neutral") {
			t.Errorf("%s: unknown preview expression did not fall back safely", ch.Name)
		}
	}
}
