package server_test

import (
	"context"
	"image/gif"
	"net/http"
	"net/http/httptest"
	"net/url"
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
		"blade": {a.Blade}, "bling": {a.Bling},
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
