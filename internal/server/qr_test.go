package server_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stuttgart-things/schmetterpause/internal/auth"
	"github.com/stuttgart-things/schmetterpause/internal/qr"
)

// sheet fetches the QR sheet with the given request headers applied.
func sheet(t *testing.T, h http.Handler, headers map[string]string) string {
	t.Helper()

	r := httptest.NewRequest(http.MethodGet, "/qr", nil)
	for k, v := range headers {
		r.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	return rec.Body.String()
}

// assertEncodes checks that the sheet does not merely print the URL but draws
// the code for it. Printing one address and encoding another is the failure
// nobody notices until somebody scans the sheet on the wall.
func assertEncodes(t *testing.T, body, target string) {
	t.Helper()

	if !strings.Contains(body, target) {
		t.Errorf("the sheet does not print %q", target)
	}

	code, err := qr.Render(target)
	if err != nil {
		t.Fatalf("Render(%q): %v", target, err)
	}
	if !strings.Contains(body, `d="`+code.Path+`"`) {
		t.Errorf("the drawn code does not encode %q", target)
	}
}

func TestSheetEncodesTheAddressTheRequestArrivedAt(t *testing.T) {
	// httptest requests carry example.com as their host.
	assertEncodes(t, sheet(t, newHandler(newMemStore()), nil), "http://example.com/#match")
}

func TestSheetFollowsTheForwardedScheme(t *testing.T) {
	// Every deployment target of this image terminates TLS in front of the
	// process, so the connection being plain says nothing about the address a
	// phone has to reach.
	body := sheet(t, newHandler(newMemStore()), map[string]string{"X-Forwarded-Proto": "https, http"})

	assertEncodes(t, body, "https://example.com/#match")
}

func TestSheetIgnoresAForwardedHost(t *testing.T) {
	// A proxy passes the Host header through. Honouring X-Forwarded-Host
	// would let whoever calls the endpoint decide where the printed code
	// points.
	body := sheet(t, newHandler(newMemStore()), map[string]string{"X-Forwarded-Host": "elsewhere.example"})

	if strings.Contains(body, "elsewhere.example") {
		t.Errorf("the sheet followed X-Forwarded-Host: %s", body)
	}
	assertEncodes(t, body, "http://example.com/#match")
}

func TestConfiguredBaseURLWins(t *testing.T) {
	cfg := testConfig()
	cfg.PublicBaseURL = "https://tischtennis.example.org"
	h := newHandlerConfig(cfg, newMemStore(), auth.Anonymous{})

	body := sheet(t, h, map[string]string{"X-Forwarded-Proto": "http"})

	assertEncodes(t, body, "https://tischtennis.example.org/#match")
	if strings.Contains(body, "example.com") {
		t.Errorf("the sheet fell back to the request host: %s", body)
	}
}

// TestTheScanTargetExistsOnTheStartPage is the part that rots silently. The
// code encodes an anchor; renaming the section it points at would leave the
// sheet on the wall scanning fine and landing at the top of the page.
func TestTheScanTargetExistsOnTheStartPage(t *testing.T) {
	store := newMemStore()
	h := newHandlerWith(store, auth.NewCookieAuthenticator(store.Identities(), testSessionKey, false))

	// Result entry is shown to recognised players only, and two of them are
	// needed before there is an opponent to pick.
	join(t, h, "Bodo")
	cookie := sessionCookie(t, join(t, h, "Anna"))

	_, fragment, ok := strings.Cut(targetFrom(t, sheet(t, h, nil)), "#")
	if !ok {
		t.Fatal("the encoded address carries no fragment, so a scan lands at the top of the page")
	}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.AddCookie(cookie)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)

	if !strings.Contains(rec.Body.String(), `id="`+fragment+`"`) {
		t.Errorf("the start page has no #%s to land on", fragment)
	}
}

// targetFrom reads the address back out of the rendered sheet, so the test
// follows the anchor the code actually carries rather than a copy of it.
func targetFrom(t *testing.T, body string) string {
	t.Helper()

	_, after, ok := strings.Cut(body, "<figcaption>")
	if !ok {
		t.Fatal("the sheet does not print the address it encodes")
	}
	target, _, ok := strings.Cut(after, "</figcaption>")
	if !ok {
		t.Fatal("the printed address is not closed off")
	}
	return strings.TrimSpace(target)
}

// TestTheSheetIsReachable keeps the printable page one click away rather than
// a URL somebody has to be told about.
func TestTheSheetIsReachable(t *testing.T) {
	if body := get(t, newHandler(newMemStore()), "/").Body.String(); !strings.Contains(body, `href="/qr"`) {
		t.Errorf("nothing on the start page links to the sheet: %s", body)
	}
}

func TestTheSheetCarriesTheMascot(t *testing.T) {
	// The sheet is printed and taped to a wall, so it is the one place the
	// drawing is worth its bytes.
	body := get(t, newHandler(newMemStore()), "/qr").Body.String()

	if !strings.Contains(body, `class="sheet-mascot`) {
		t.Errorf("the sheet has no illustration: %s", body)
	}
	// Inlined, not linked: the drawing itself has to be in the response.
	// Asserted on the recolourable blade rather than on "<svg", because the
	// QR code on this page is an svg too and would satisfy a looser check.
	if !strings.Contains(body, `id="paddle-face"`) {
		t.Errorf("the sheet does not carry the illustration itself: %s", body)
	}
}

// TestTheMascotIsInlinedSoItCanBeRecoloured is the reason the mark became a
// vector at all. An SVG behind <img src> is an isolated document that the
// page's CSS cannot reach, so a --paddle set anywhere outside would never
// arrive. See issue #64.
func TestTheMascotIsInlinedSoItCanBeRecoloured(t *testing.T) {
	h := newHandler(newMemStore())

	for _, path := range []string{"/", "/qr"} {
		body := get(t, h, path).Body.String()

		if strings.Contains(body, "/static/img/mascot.svg") {
			t.Errorf("%s links the mascot instead of inlining it", path)
		}
		if !strings.Contains(body, "var(--paddle, #c82828)") {
			t.Errorf("%s does not carry the recolourable blade: %s", path, body)
		}
	}
}
