package server_test

import (
	"context"
	"net/http"
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
