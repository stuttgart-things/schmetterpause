package server_test

import (
	"context"
	"image/gif"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/stuttgart-things/schmetterpause/internal/avatar"
	"github.com/stuttgart-things/schmetterpause/internal/templates"
)

// cream and pink, so the figure is told apart from the default by its fur and
// by a blade the id would rarely derive: pink is paddle-4.
var chosen = avatar.Avatar{Fur: "cream", Headdress: "lapis", Handle: "wood-dark", Blade: "pink", Bling: "chain"}

// TestTheProfileShowsTheChosenFigure: the stored figure, its blade's class on
// the box, and the bling it wears (docs/adr/0029).
func TestTheProfileShowsTheChosenFigure(t *testing.T) {
	h, store, anna, _ := twoBrowsers(t)
	id := opponentID(t, store, "Anna")
	if err := store.Avatars().Put(context.Background(), uuid.MustParse(id), chosen); err != nil {
		t.Fatal(err)
	}

	body := fragment(t, h, "/players/"+id, anna).Body.String()
	for _, want := range []string{
		`class="page-mascot paddle-4"`,
		"--fur:#f2e3c6",
		"--headdress:#3783e6",
		"--paddle:#d64f9c",
		`data-bling="chain"`,
		`data-face="neutral"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the profile does not contain %q", want)
		}
	}
}

// TestAProfileWithoutAChoiceLooksAsBefore: nobody looks different the day
// figures arrive. The box carries the class PaddleClass has always derived,
// which is also what the pipeline's check compares between the start page
// and the profile.
func TestAProfileWithoutAChoiceLooksAsBefore(t *testing.T) {
	h, store, anna, _ := twoBrowsers(t)
	id := opponentID(t, store, "Anna")

	body := fragment(t, h, "/players/"+id, anna).Body.String()
	if want := `class="page-mascot ` + templates.PaddleClass(id) + `"`; !strings.Contains(body, want) {
		t.Errorf("the profile does not carry %s", want)
	}
	if !strings.Contains(body, "--fur:#f89828") || strings.Contains(body, "sthings-bling") {
		t.Error("the default figure is not the drawing")
	}
	// The pipeline's smoke test looks for this on every page with a mascot.
	if !strings.Contains(body, "var(--paddle, #c82828)") {
		t.Error("the blade lost its fallback")
	}
}

// TestTheStartPageShowsTheReadersFigure, and the drawing to a browser nobody
// is recognised in.
func TestTheStartPageShowsTheReadersFigure(t *testing.T) {
	h, store, anna, _ := twoBrowsers(t)
	if err := store.Avatars().Put(context.Background(), uuid.MustParse(opponentID(t, store, "Anna")), chosen); err != nil {
		t.Fatal(err)
	}

	if body := fragment(t, h, "/", anna).Body.String(); !strings.Contains(body, "--fur:#f2e3c6") {
		t.Error("the start page does not show Anna's figure")
	}

	anon := get(t, h, "/").Body.String()
	if strings.Contains(anon, `class="sthings"`) || strings.Contains(anon, `class="page-mascot paddle-`) {
		t.Error("the start page dresses a browser nobody is recognised in")
	}
	if !strings.Contains(anon, "var(--paddle, #c82828)") {
		t.Error("the signed-out start page lost the mascot")
	}
}

// TestTheFigureFileCarriesEveryFace: what the live score and the Zählwerk
// load once per match and switch by attribute.
func TestTheFigureFileCarriesEveryFace(t *testing.T) {
	h, store, _, _ := twoBrowsers(t)
	id := opponentID(t, store, "Anna")
	if err := store.Avatars().Put(context.Background(), uuid.MustParse(id), chosen); err != nil {
		t.Fatal(err)
	}

	rec := get(t, h, "/avatars/"+id+".svg?face=happy")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "image/svg+xml" {
		t.Errorf("Content-Type = %q", got)
	}
	if got := rec.Header().Get("Content-Security-Policy"); !strings.Contains(got, "default-src 'none'") {
		t.Errorf("Content-Security-Policy = %q, want nothing but inline style", got)
	}
	body := rec.Body.String()
	if !strings.HasPrefix(body, "<svg") || !strings.Contains(body, `data-face="happy"`) || !strings.Contains(body, "--fur:#f2e3c6") {
		t.Errorf("not Anna's figure with the happy face: %.120s", body)
	}
	for _, f := range avatar.Faces {
		if !strings.Contains(body, `<g class="sthings-face" data-face="`+f.Key+`"`) {
			t.Errorf("face %q missing", f.Key)
		}
	}
}

func TestTheFigureFileOfNobody(t *testing.T) {
	h, _, _, _ := twoBrowsers(t)
	for _, path := range []string{
		"/avatars/" + uuid.NewString() + ".svg", // nobody by that id
		"/avatars/not-an-id.svg",
		"/avatars/" + uuid.NewString(), // no extension
	} {
		if rec := get(t, h, path); rec.Code != http.StatusNotFound {
			t.Errorf("%s: status = %d, want 404", path, rec.Code)
		}
	}
}

// TestARandomFigureIsNotCached: one per demo match, different every time.
func TestARandomFigureIsNotCached(t *testing.T) {
	h, _, _, _ := twoBrowsers(t)

	rec := get(t, h, "/avatars/random.svg")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", got)
	}

	seen := map[string]bool{}
	for range 20 {
		seen[get(t, h, "/avatars/random.svg").Body.String()] = true
	}
	if len(seen) < 2 {
		t.Error("twenty random figures were all the same")
	}
}

func figureForm(a avatar.Avatar) url.Values {
	return url.Values{
		"fur": {a.Fur}, "headdress": {a.Headdress}, "handle": {a.Handle},
		"blade": {a.Blade}, "bling": {a.Bling}, "ballColor": {a.BallColor},
	}
}

// TestTheEditorIsOnYourOwnProfileOnly: nobody else's, and not for a browser
// nobody is recognised in.
func TestTheEditorIsOnYourOwnProfileOnly(t *testing.T) {
	h, store, anna, bodo := twoBrowsers(t)
	id := opponentID(t, store, "Anna")

	own := fragment(t, h, "/players/"+id, anna).Body.String()
	if !strings.Contains(own, `id="figure-editor"`) {
		t.Fatal("Anna's own profile has no editor")
	}
	// What she wears is what is checked: the default, with her derived blade.
	blade := avatar.Blades[avatar.BladeIndex(id)].Key
	for _, want := range []string{
		`name="fur" value="orange" checked`,
		`name="headdress" value="fur" checked`,
		`name="blade" value="` + blade + `" checked`,
		`name="bling" value="" checked`,
		`style="background:#f2e3c6;"`, // the swatches carry their colour
	} {
		if !strings.Contains(own, want) {
			t.Errorf("the editor does not contain %q", want)
		}
	}

	if strings.Contains(fragment(t, h, "/players/"+id, bodo).Body.String(), "figure-editor") {
		t.Error("Bodo sees an editor on Anna's profile")
	}
	if strings.Contains(get(t, h, "/players/"+id).Body.String(), "figure-editor") {
		t.Error("a stranger sees an editor on Anna's profile")
	}
}

func TestThePreviewStoresNothing(t *testing.T) {
	h, store, anna, _ := twoBrowsers(t)
	id := uuid.MustParse(opponentID(t, store, "Anna"))

	rec := fragment(t, h, "/fragments/figure-preview?"+figureForm(chosen).Encode(), anna)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "--fur:#f2e3c6") {
		t.Fatalf("preview: status %d, %.120s", rec.Code, rec.Body.String())
	}
	if _, err := store.Avatars().Get(context.Background(), id); err == nil {
		t.Error("the preview stored the figure")
	}

	bad := figureForm(chosen)
	bad.Set("fur", "plaid")
	if rec := fragment(t, h, "/fragments/figure-preview?"+bad.Encode(), anna); rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("an unknown fur: status %d, want 422", rec.Code)
	}
}

// TestSavingStoresYourOwnFigure, from the session and never from the form,
// and puts the new figure beside the name in the same response.
func TestSavingStoresYourOwnFigure(t *testing.T) {
	h, store, anna, _ := twoBrowsers(t)
	annaID := uuid.MustParse(opponentID(t, store, "Anna"))
	bodoID := uuid.MustParse(opponentID(t, store, "Bodo"))

	form := figureForm(chosen)
	form.Set("player_id", bodoID.String()) // not a field the handler reads
	rec := postAs(t, h, "/figure", anna, form)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Gespeichert") {
		t.Error("the editor does not say it saved")
	}
	if !strings.Contains(body, `id="profile-figure"`) || !strings.Contains(body, `hx-swap-oob="true"`) {
		t.Error("the figure beside the name is not replaced out of band")
	}

	if got, err := store.Avatars().Get(context.Background(), annaID); err != nil || got != chosen {
		t.Errorf("stored for Anna: %+v, %v; want %+v", got, err, chosen)
	}
	if _, err := store.Avatars().Get(context.Background(), bodoID); err == nil {
		t.Error("Bodo's figure was changed by Anna's form")
	}
}

func TestSavingRefusesAnUnknownChoice(t *testing.T) {
	h, store, anna, _ := twoBrowsers(t)
	form := figureForm(chosen)
	form.Set("blade", "red") // the drawing's red is not a choice

	if rec := postAs(t, h, "/figure", anna, form); rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422", rec.Code)
	}
	if _, err := store.Avatars().Get(context.Background(), uuid.MustParse(opponentID(t, store, "Anna"))); err == nil {
		t.Error("an invalid figure was stored")
	}
}

func TestResetFallsBackToTheDefault(t *testing.T) {
	h, store, anna, _ := twoBrowsers(t)
	id := uuid.MustParse(opponentID(t, store, "Anna"))
	if err := store.Avatars().Put(context.Background(), id, chosen); err != nil {
		t.Fatal(err)
	}

	for range 2 { // twice is still reset
		rec := postAs(t, h, "/figure/reset", anna, url.Values{})
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Zurückgesetzt") {
			t.Fatalf("status %d: %.200s", rec.Code, rec.Body.String())
		}
	}
	if _, err := store.Avatars().Get(context.Background(), id); err == nil {
		t.Error("the figure is still stored after a reset")
	}
}

func TestRollingSuggestsWithoutStoring(t *testing.T) {
	h, store, anna, _ := twoBrowsers(t)

	rec := fragment(t, h, "/fragments/figure-roll", anna)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "noch nicht gespeichert") {
		t.Fatalf("status %d: %.200s", rec.Code, rec.Body.String())
	}
	if _, err := store.Avatars().Get(context.Background(), uuid.MustParse(opponentID(t, store, "Anna"))); err == nil {
		t.Error("rolling stored a figure")
	}
}

// TestTheEditorNeedsAPlayer: nothing to dress without one.
func TestTheEditorNeedsAPlayer(t *testing.T) {
	h, _, _, _ := twoBrowsers(t)
	for _, rec := range []*httptest.ResponseRecorder{
		postAs(t, h, "/figure", nil, figureForm(chosen)),
		postAs(t, h, "/figure/reset", nil, url.Values{}),
		fragment(t, h, "/fragments/figure-roll", nil),
	} {
		if rec.Code < 400 {
			t.Errorf("status = %d without a player, want a refusal", rec.Code)
		}
	}
}

// TestTheFigureAsGIF: the same figure for an LED panel, which renders no SVG
// (docs/adr/0030). Without a face it cheers, with one it is a still, and the
// size is held to what a panel can use.
func TestTheFigureAsGIF(t *testing.T) {
	h, store, _, _ := twoBrowsers(t)
	id := opponentID(t, store, "Anna")
	if err := store.Avatars().Put(context.Background(), uuid.MustParse(id), chosen); err != nil {
		t.Fatal(err)
	}

	rec := get(t, h, "/avatars/"+id+".gif")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "image/gif" {
		t.Errorf("Content-Type = %q", got)
	}
	g, err := gif.DecodeAll(rec.Body)
	if err != nil {
		t.Fatalf("not a GIF: %v", err)
	}
	if len(g.Image) != len(avatar.Cheer) {
		t.Errorf("%d frames, want the cheer's %d", len(g.Image), len(avatar.Cheer))
	}
	if got := g.Image[0].Bounds().Dx(); got != avatar.DefaultGIFSize {
		t.Errorf("%d pixels wide, want %d", got, avatar.DefaultGIFSize)
	}
	// Anna's fur, #f2e3c6, is in the picture: it is her figure.
	anna := false
	for _, c := range g.Image[0].Palette {
		if r, gr, b, _ := c.RGBA(); r>>8 == 0xf2 && gr>>8 == 0xe3 && b>>8 == 0xc6 {
			anna = true
		}
	}
	if !anna {
		t.Error("Anna's fur is not in the GIF's colours")
	}

	still, err := gif.DecodeAll(get(t, h, "/avatars/"+id+".gif?face=happy&size=32").Body)
	if err != nil {
		t.Fatal(err)
	}
	if len(still.Image) != 1 || still.Image[0].Bounds().Dx() != 32 {
		t.Errorf("?face=happy&size=32: %d frames of %v, want one of 32", len(still.Image), still.Image[0].Bounds())
	}

	if rec := get(t, h, "/avatars/"+uuid.NewString()+".gif"); rec.Code != http.StatusNotFound {
		t.Errorf("nobody's GIF: status = %d, want 404", rec.Code)
	}
	rnd := get(t, h, "/avatars/random.gif")
	if rnd.Code != http.StatusOK || rnd.Header().Get("Cache-Control") != "no-store" {
		t.Errorf("random.gif: status %d, Cache-Control %q", rnd.Code, rnd.Header().Get("Cache-Control"))
	}
}

func TestCharacterSwitchDoesNotStoreAndResetsMaterials(t *testing.T) {
	h, store, anna, _ := twoBrowsers(t)
	id := uuid.MustParse(opponentID(t, store, "Anna"))
	for _, ch := range avatar.Characters {
		t.Run(ch.Name, func(t *testing.T) {
			q := url.Values{"character": {ch.Key}, "blade": {"pink"}, "fur": {"white"}, "bling": {"chain"}}
			rec := fragment(t, h, "/fragments/figure-character?"+q.Encode(), anna)
			if rec.Code != http.StatusOK {
				t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
			}
			body := rec.Body.String()
			if !strings.Contains(body, `name="character" value="`+ch.Key+`" checked`) {
				t.Error("new character is not selected")
			}
			if !strings.Contains(body, `name="blade" value="pink" checked`) {
				t.Error("switch lost blade colour")
			}
			if ch.Key != "" {
				if !strings.Contains(body, `name="fur" value="" checked`) {
					t.Error("new character did not use original materials")
				}
				if strings.Contains(body, `name="bling" value="chain"`) {
					t.Error("new character offers jewellery drawn for sthings")
				}
			}
			if _, err := store.Avatars().Get(context.Background(), id); err == nil {
				t.Error("switch stored a suggestion")
			}
		})
	}
	if rec := fragment(t, h, "/fragments/figure-character?character=unknown&blade=pink", anna); rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("unknown character: %d", rec.Code)
	}
	if rec := get(t, h, "/fragments/figure-character?character=pip&blade=pink"); rec.Code < 400 {
		t.Errorf("anonymous switch: %d", rec.Code)
	}
}

func TestSavedCharactersReachProfileSVGAndGIF(t *testing.T) {
	h, store, anna, _ := twoBrowsers(t)
	id := uuid.MustParse(opponentID(t, store, "Anna"))
	for _, ch := range avatar.Characters {
		t.Run(ch.Name, func(t *testing.T) {
			a := avatar.DefaultForCharacter(ch.Key, id.String())
			a.Blade = "pink"
			form := figureForm(a)
			form.Set("character", a.Character)
			rec := postAs(t, h, "/figure", anna, form)
			if rec.Code != http.StatusOK {
				t.Fatalf("save: %d: %s", rec.Code, rec.Body.String())
			}
			if got, err := store.Avatars().Get(context.Background(), id); err != nil || got != a {
				t.Fatalf("saved %+v, %v; want %+v", got, err, a)
			}
			svg := get(t, h, "/avatars/"+id.String()+".svg?face=happy").Body.String()
			if ch.Key != "" && !strings.Contains(svg, `data-character="`+ch.Key+`"`) {
				t.Error("SVG serves the wrong character")
			}
			if strings.Count(svg, `class="sthings-face"`) != 6 {
				t.Error("live SVG lost expressions")
			}
			profile := fragment(t, h, "/players/"+id.String(), anna).Body.String()
			start := strings.Index(profile, `id="profile-figure"`)
			if start < 0 {
				t.Fatal("no profile figure")
			}
			end := strings.Index(profile[start:], "</svg>")
			if end < 0 {
				t.Fatal("no profile figure")
			}
			if ch.Key != "" && !strings.Contains(profile[start:start+end], `data-character="`+ch.Key+`"`) {
				t.Error("profile serves the wrong character")
			}
			g, err := gif.DecodeAll(get(t, h, "/avatars/"+id.String()+".gif").Body)
			if err != nil {
				t.Fatal(err)
			}
			if len(g.Image) != len(avatar.Cheer) {
				t.Error("GIF lost cheer frames")
			}
		})
	}
	bad := figureForm(chosen)
	bad.Set("character", `unknown"><script>`)
	if rec := postAs(t, h, "/figure", anna, bad); rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("invalid character save: %d", rec.Code)
	}
}

func TestExpressionGalleryFollowsUnsavedCharacterAndColours(t *testing.T) {
	h, store, anna, bodo := twoBrowsers(t)
	id := uuid.MustParse(opponentID(t, store, "Anna"))
	stored := avatar.Default(id.String())
	if err := store.Avatars().Put(context.Background(), id, stored); err != nil {
		t.Fatal(err)
	}
	check := func(t *testing.T, body, character string) {
		t.Helper()
		start := strings.Index(body, `id="figure-expressions"`)
		if start < 0 {
			t.Fatal("expression gallery missing")
		}
		end := strings.Index(body[start:], "</section>")
		if end < 0 {
			t.Fatal("unclosed expression gallery")
		}
		gallery := body[start : start+end]
		cards := regexp.MustCompile(`(?s)<figure[^>]*data-expression="([^"]+)"[^>]*>(.*?)</figure>`).FindAllStringSubmatch(gallery, -1)
		if len(cards) != len(avatar.Faces) {
			t.Fatalf("%d previews, want six", len(cards))
		}
		for i, card := range cards {
			face := avatar.Faces[i]
			if card[1] != face.Key || !strings.Contains(card[2], face.Name) {
				t.Errorf("wrong expression label: %s", card[1])
			}
			if strings.Count(card[2], `class="sthings-face"`) != 1 || !strings.Contains(card[2], `<g class="sthings-face" data-face="`+face.Key+`"`) {
				t.Errorf("%s does not show exactly its own face", face.Key)
			}
			if character != "" && !strings.Contains(card[2], `data-character="`+character+`"`) {
				t.Errorf("%s shows the wrong character", face.Key)
			}
		}
	}
	own := fragment(t, h, "/players/"+id.String(), anna).Body.String()
	check(t, own, "")
	if strings.Contains(fragment(t, h, "/players/"+id.String(), bodo).Body.String(), `id="figure-expressions"`) || strings.Contains(get(t, h, "/players/"+id.String()).Body.String(), `id="figure-expressions"`) {
		t.Error("private editor gallery is visible on somebody else's profile")
	}
	for _, ch := range avatar.Characters {
		t.Run(ch.Name, func(t *testing.T) {
			q := url.Values{"character": {ch.Key}, "blade": {"pink"}}
			switched := fragment(t, h, "/fragments/figure-character?"+q.Encode(), anna)
			if switched.Code != http.StatusOK {
				t.Fatalf("switch: %d", switched.Code)
			}
			check(t, switched.Body.String(), ch.Key)
			a := avatar.DefaultForCharacter(ch.Key, id.String())
			a.Fur = "white"
			a.Handle = "blue"
			a.Headdress = "lapis"
			a.Blade = "pink"
			q = figureForm(a)
			q.Set("character", ch.Key)
			q.Set("face", "angry")
			rec := fragment(t, h, "/fragments/figure-preview?"+q.Encode(), anna)
			if rec.Code != http.StatusOK {
				t.Fatalf("preview: %d", rec.Code)
			}
			body := rec.Body.String()
			check(t, body, ch.Key)
			if !strings.Contains(body, `hx-swap-oob="true"`) {
				t.Error("colour preview does not refresh the gallery")
			}
			for _, colour := range []string{"--fur:#f7f5f0", "--handle:#2f6fd0", "--paddle:#d64f9c"} {
				if strings.Count(body, colour) != 8 {
					t.Errorf("%s was not applied to main preview, selected thumbnail and six expressions", colour)
				}
			}
			if got, err := store.Avatars().Get(context.Background(), id); err != nil || got != stored {
				t.Errorf("gallery changed the saved figure: %+v, %v", got, err)
			}
		})
	}
}

func TestVexPaletteOptionsAndRequestsAreRestricted(t *testing.T) {
	h, store, anna, _ := twoBrowsers(t)
	id := uuid.MustParse(opponentID(t, store, "Anna"))
	banned := []string{"orange", "ginger", "cinnamon", "gold", "cream"}
	rec := fragment(t, h, "/fragments/figure-character?character=vex&blade=pink", anna)
	body := rec.Body.String()
	if rec.Code != http.StatusOK {
		t.Fatal(rec.Code)
	}
	for _, key := range banned {
		if strings.Contains(body, `name="fur" value="`+key+`"`) {
			t.Errorf("Vex still offers %s", key)
		}
	}
	for _, key := range []string{"", "sand", "white", "chocolate"} {
		if !strings.Contains(body, `name="fur" value="`+key+`"`) {
			t.Errorf("Vex lost provisional/original coat %q", key)
		}
	}
	for _, key := range banned {
		a := avatar.DefaultForCharacter("vex", id.String())
		a.Fur = key
		form := figureForm(a)
		form.Set("character", "vex")
		if r := postAs(t, h, "/figure", anna, form); r.Code != http.StatusUnprocessableEntity {
			t.Errorf("saving Vex %s: %d", key, r.Code)
		}
		if r := fragment(t, h, "/fragments/figure-preview?"+form.Encode(), anna); r.Code != http.StatusUnprocessableEntity {
			t.Errorf("previewing Vex %s: %d", key, r.Code)
		}
	}
	if _, err := store.Avatars().Get(context.Background(), id); err == nil {
		t.Error("forbidden palette request stored a figure")
	}
	// Reads of stale persisted colours fall back visually, without rewriting data.
	stale := avatar.DefaultForCharacter("vex", id.String())
	stale.Fur = "ginger"
	if err := store.Avatars().Put(context.Background(), id, stale); err != nil {
		t.Fatal(err)
	}
	profile := fragment(t, h, "/players/"+id.String(), anna).Body.String()
	if !strings.Contains(profile, `name="fur" value="" checked`) {
		t.Error("stored ginger Vex did not fall back to original grey")
	}
	if strings.Contains(profile, "--fur:#d9622b") {
		t.Error("a stale ginger coat reached the preview")
	}
	if got, err := store.Avatars().Get(context.Background(), id); err != nil || got != stale {
		t.Error("read mutated persisted choices")
	}
	for _, ch := range avatar.Characters {
		if ch.Key == "vex" {
			continue
		}
		other := fragment(t, h, "/fragments/figure-character?character="+ch.Key+"&blade=pink", anna).Body.String()
		for _, key := range banned {
			if !strings.Contains(other, `name="fur" value="`+key+`"`) {
				t.Errorf("%s lost %s", ch.Name, key)
			}
		}
	}
}

func TestBleckiJewelleryPersistsPreviewsAndResets(t *testing.T) {
	h, store, anna, _ := twoBrowsers(t)
	id := uuid.MustParse(opponentID(t, store, "Anna"))
	for _, bling := range avatar.BlingPalette("vex") {
		a := avatar.DefaultForCharacter("vex", id.String())
		a.Fur = "white"
		a.Handle = "red"
		a.Blade = "pink"
		a.Bling = bling.Key
		q := figureForm(a)
		q.Set("character", "vex")
		preview := fragment(t, h, "/fragments/figure-preview?"+q.Encode(), anna)
		if preview.Code != 200 {
			t.Fatalf("preview %s: %d", bling.Key, preview.Code)
		}
		if !strings.Contains(preview.Body.String(), `id="character-thumbnail-vex"`) {
			t.Error("selected thumbnail is not refreshed")
		}
		if r := postAs(t, h, "/figure", anna, q); r.Code != 200 {
			t.Fatalf("save %s: %d", bling.Key, r.Code)
		}
		if got, err := store.Avatars().Get(context.Background(), id); err != nil || got != a {
			t.Fatalf("stored %+v: %v", got, err)
		}
		profile := fragment(t, h, "/players/"+id.String(), anna).Body.String()
		thumb := regexp.MustCompile(`(?s)<span id="character-thumbnail-vex"[^>]*>(.*?)</span>`).FindStringSubmatch(profile)
		if len(thumb) < 2 || !strings.Contains(thumb[1], "--fur:#f7f5f0") || !strings.Contains(thumb[1], "--handle:#c82828") || !strings.Contains(thumb[1], "--paddle:#d64f9c") {
			t.Fatal("selected thumbnail lost stored colours")
		}
		svg := get(t, h, "/avatars/"+id.String()+".svg").Body.String()
		if bling.Key != "" && (!strings.Contains(thumb[1], `data-bling="`+bling.Key+`"`) || !strings.Contains(svg, `data-bling="`+bling.Key+`"`)) {
			t.Error("jewellery missing from thumbnail or public SVG")
		}
		if r := get(t, h, "/avatars/"+id.String()+".gif?face=angry"); r.Code != 200 {
			t.Error("jewellery breaks LED rendering")
		}
	}
	if r := postAs(t, h, "/figure/reset", anna, nil); r.Code != 200 {
		t.Fatal(r.Code)
	}
	if _, err := store.Avatars().Get(context.Background(), id); err == nil {
		t.Error("reset kept jewellery")
	}
	stale := avatar.DefaultForCharacter("vex", id.String())
	stale.Fur = "cinnamon"
	if err := store.Avatars().Put(context.Background(), id, stale); err != nil {
		t.Fatal(err)
	}
	profile := fragment(t, h, "/players/"+id.String(), anna).Body.String()
	if !strings.Contains(profile, `name="fur" value="" checked`) || strings.Contains(profile, `name="fur" value="cinnamon"`) {
		t.Error("stored cinnamon does not show Original")
	}
	if !strings.Contains(profile, "blecki") || strings.Contains(profile, ">Vex<") || strings.Contains(profile, `value="pip"`) || strings.Contains(profile, `value="pogo"`) || strings.Contains(profile, `value="kiki"`) {
		t.Error("release character selection is wrong")
	}
}

func TestBleckiRetiredAccessoriesAreHiddenRejectedAndReadSafely(t *testing.T) {
	h, store, anna, _ := twoBrowsers(t)
	id := uuid.MustParse(opponentID(t, store, "Anna"))
	for _, key := range []string{"towel", "captain", "bandana", "moon", "wolf-tooth"} {
		a := avatar.DefaultForCharacter("vex", id.String())
		a.Bling = key
		if err := store.Avatars().Put(context.Background(), id, a); err != nil {
			t.Fatal(err)
		}
		body := fragment(t, h, "/players/"+id.String(), anna).Body.String()
		for _, retired := range []string{"towel", "captain", "bandana", "moon", "wolf-tooth"} {
			if strings.Contains(body, `name="bling" value="`+retired+`"`) || strings.Contains(body, `data-bling="`+retired+`"`) {
				t.Errorf("retired %s remains visible", retired)
			}
		}
		if !strings.Contains(body, `name="bling" value="" checked`) || !strings.Contains(body, `name="bling" value="ball"`) {
			t.Error("expected none and ball choices")
		}
		q := figureForm(a)
		q.Set("character", "vex")
		if r := fragment(t, h, "/fragments/figure-preview?"+q.Encode(), anna); r.Code != http.StatusUnprocessableEntity {
			t.Errorf("retired preview %s: %d", key, r.Code)
		}
		if r := postAs(t, h, "/figure", anna, q); r.Code != http.StatusUnprocessableEntity {
			t.Errorf("retired save %s: %d", key, r.Code)
		}
		if got, err := store.Avatars().Get(context.Background(), id); err != nil || got != a {
			t.Error("reading/rejecting retired accessory mutated stored data")
		}
	}
}

func TestBleckiBallColourControlPersistenceAndFallback(t *testing.T) {
	h, store, anna, _ := twoBrowsers(t)
	id := uuid.MustParse(opponentID(t, store, "Anna"))
	for _, bling := range []string{"", "ball"} {
		a := avatar.DefaultForCharacter("vex", id.String())
		a.Bling = bling
		a.BallColor = "orange"
		q := figureForm(a)
		q.Set("character", "vex")
		r := fragment(t, h, "/fragments/figure-preview?"+q.Encode(), anna)
		if r.Code != 200 {
			t.Fatal(r.Code)
		}
		body := r.Body.String()
		visible := strings.Contains(body, "<legend>Ballfarbe</legend>")
		if visible != (bling == "ball") {
			t.Errorf("ball colour control visibility for %q", bling)
		}
		if !strings.Contains(body, `id="figure-ball-color"`) || !strings.Contains(body, `hx-swap-oob="true"`) {
			t.Error("accessory changes must refresh the control")
		}
		if bling != "ball" && !strings.Contains(body, `type="hidden" name="ballColor" value="orange"`) {
			t.Error("dormant colour is lost")
		}
		if r := postAs(t, h, "/figure", anna, q); r.Code != 200 {
			t.Fatal(r.Code)
		}
		if got, err := store.Avatars().Get(context.Background(), id); err != nil || got.BallColor != "orange" || got.Bling != bling {
			t.Errorf("save colour: %+v %v", got, err)
		}
	}
	for _, key := range []string{"", "orange", "yellow", "invalid"} {
		a := avatar.DefaultForCharacter("vex", id.String())
		a.Bling = "ball"
		a.BallColor = key
		q := figureForm(a)
		q.Set("character", "vex")
		if r := postAs(t, h, "/figure", anna, q); r.Code != 200 {
			t.Fatal(r.Code)
		}
		got, err := store.Avatars().Get(context.Background(), id)
		if err != nil || got.BallColor != avatar.NormalizeBallColor(key) {
			t.Errorf("colour fallback: %+v %v", got, err)
		}
		body := get(t, h, "/avatars/"+id.String()+".svg").Body.String()
		if !strings.Contains(body, "--ball:"+avatar.BallTone(key).Hex) {
			t.Error("saved colour missing from public SVG")
		}
	}
	stale := avatar.DefaultForCharacter("vex", id.String())
	stale.Bling = "ball"
	stale.BallColor = "obsolete"
	if err := store.Avatars().Put(context.Background(), id, stale); err != nil {
		t.Fatal(err)
	}
	body := fragment(t, h, "/players/"+id.String(), anna).Body.String()
	if !strings.Contains(body, `name="ballColor" value="" checked`) {
		t.Error("stale stored colour does not show white")
	}
	if got, err := store.Avatars().Get(context.Background(), id); err != nil || got != stale {
		t.Error("read rewrites stale colour")
	}
	if r := postAs(t, h, "/figure/reset", anna, nil); r.Code != 200 {
		t.Fatal(r.Code)
	}
	body = fragment(t, h, "/players/"+id.String(), anna).Body.String()
	if strings.Contains(body, "<legend>Ballfarbe</legend>") {
		t.Error("reset must hide ball colours")
	}
}
