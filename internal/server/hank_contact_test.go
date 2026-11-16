package server_test

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"image"
	"image/color"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/srwiley/oksvg"
	"github.com/srwiley/rasterx"
	"github.com/stuttgart-things/schmetterpause/internal/avatar"
)

type contactPath struct {
	D    string `xml:"d,attr"`
	Fill string `xml:"fill,attr"`
}
type contactGroup struct {
	Part   string         `xml:"data-pip-part,attr"`
	Style  string         `xml:"style,attr"`
	Paths  []contactPath  `xml:"path"`
	Groups []contactGroup `xml:"g"`
}

func contactFind(g contactGroup, part string) (contactGroup, bool) {
	if g.Part == part {
		return g, true
	}
	for _, c := range g.Groups {
		if r, ok := contactFind(c, part); ok {
			return r, true
		}
	}
	return contactGroup{}, false
}
func contactRaster(t *testing.T, svg string) *image.RGBA {
	t.Helper()
	icon, err := oksvg.ReadIconStream(strings.NewReader(svg), oksvg.StrictErrorMode)
	if err != nil {
		t.Fatal(err)
	}
	icon.SetTarget(0, 0, 420, 420)
	img := image.NewRGBA(image.Rect(0, 0, 420, 420))
	icon.Draw(rasterx.NewDasher(420, 420, rasterx.NewScannerGV(420, 420, img, img.Bounds())), 1)
	return img
}
func contactMask(t *testing.T, p []contactPath) *image.RGBA {
	var s strings.Builder
	s.WriteString(`<svg viewBox="0 0 420 420">`)
	for _, p := range p {
		s.WriteString(`<path fill="#ffffff" fill-rule="evenodd" d="` + p.D + `"/>`)
	}
	s.WriteString(`</svg>`)
	return contactRaster(t, s.String())
}
func contactInside(img *image.RGBA, x, y float64) bool {
	ix, iy := int(math.Floor(x)), int(math.Floor(y))
	return image.Pt(ix, iy).In(img.Bounds()) && img.RGBAAt(ix, iy).A >= 192
}
func contactHTTP(t *testing.T, url string) []byte {
	t.Helper()
	r, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Body.Close()
	if r.StatusCode != 200 {
		t.Fatal(url, r.Status)
	}
	b, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// This checks the real HTTP response, including its paint order, rather than
// reading repository SVGs. Set SP_HANK_CONTACT_URL to additionally test 8080.
func TestHankContactThroughHTTP(t *testing.T) {
	h, store, _, _ := twoBrowsers(t)
	id := uuid.MustParse(opponentID(t, store, "Anna"))
	a := avatar.DefaultForCharacter("pip", id.String())
	a.Fur = "blue-grey"
	a.Handle = "red"
	a.Blade = "light-blue"
	if err := store.Avatars().Put(t.Context(), id, a); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h)
	defer srv.Close()
	t.Run("handler", func(t *testing.T) {
		validateHankContact(t, contactHTTP(t, srv.URL+"/avatars/"+id.String()+".svg?face=neutral"))
	})
	if live := os.Getenv("SP_HANK_CONTACT_URL"); live != "" {
		t.Run("running-app", func(t *testing.T) { validateHankContact(t, contactHTTP(t, live)) })
	}
}

func validateHankContact(t *testing.T, raw []byte) {
	t.Helper()
	var root contactGroup
	if err := xml.Unmarshal(raw, &root); err != nil {
		t.Fatal(err)
	}
	racket, ok := contactFind(root, "racket")
	if !ok || len(racket.Paths) != 5 {
		t.Fatal("HTTP SVG needs the single shaft, upper binding and capped lower end")
	}
	hand, ok := contactFind(racket, "flipper")
	if !(ok && len(hand.Paths) == 3) {
		t.Fatal("HTTP SVG needs one closed mitten and one fold")
	}
	joined, ok := contactFind(racket, "joined-arm-fist")
	if !(ok && len(joined.Paths) == 2) {
		t.Fatal("HTTP SVG needs one shared arm/fist silhouette with a single outside contour")
	}
	text := string(raw)
	gripIndex := strings.Index(text, `d="`+racket.Paths[1].D+`"`)
	handIndex := strings.Index(text, `data-pip-part="flipper"`)
	bladeIndex := strings.Index(text, `data-pip-part="blade"`)
	if gripIndex < 0 || !(gripIndex < handIndex && handIndex < bladeIndex) {
		t.Fatal("paint order must be shaft -> mitten -> blade")
	}
	joinedIndex := strings.Index(text, `data-pip-part="joined-arm-fist"`)
	if joinedIndex < 0 || !(joinedIndex < gripIndex && gripIndex < handIndex) {
		t.Fatal("joined arm and fist must share one contour behind shaft and finger side")
	}
	grip := contactMask(t, racket.Paths)
	colour := contactMask(t, racket.Paths[1:2])
	flipper := contactMask(t, hand.Paths)
	fur := contactMask(t, hand.Paths[1:2])
	fold := contactMask(t, hand.Paths[2:])
	blade, _ := contactFind(racket, "blade")
	bladeMask := contactMask(t, blade.Paths)
	var points [][2]float64
	cx, cy := 0., 0.
	for y := 0; y < 420; y++ {
		for x := 0; x < 420; x++ {
			if contactInside(grip, float64(x)+.5, float64(y)+.5) {
				q := [2]float64{float64(x) + .5, float64(y) + .5}
				points = append(points, q)
				cx += q[0]
				cy += q[1]
			}
		}
	}
	cx /= float64(len(points))
	cy /= float64(len(points))
	xx, yy, xy := 0., 0., 0.
	for _, q := range points {
		dx, dy := q[0]-cx, q[1]-cy
		xx += dx * dx
		yy += dy * dy
		xy += dx * dy
	}
	theta := .5 * math.Atan2(2*xy, xx-yy)
	ux, uy := math.Cos(theta), math.Sin(theta)
	if uy < 0 {
		ux, uy = -ux, -uy
	}
	angle := math.Atan2(-ux, uy) * 180 / math.Pi
	if math.Abs(angle-13.42) > 2 {
		t.Fatalf("shaft axis %.3f", angle)
	}
	interval := func(mask *image.RGBA) (float64, float64) {
		lo, hi := math.Inf(1), math.Inf(-1)
		for tt := -160.; tt <= 160; tt += .1 {
			if contactInside(mask, cx+ux*tt, cy+uy*tt) {
				lo = math.Min(lo, tt)
				hi = math.Max(hi, tt)
			}
		}
		return lo, hi
	}
	_, headEnd := interval(bladeMask)
	handStart, handEnd := interval(flipper)
	_, shaftEnd := interval(colour)
	neck, lower := handStart-headEnd, shaftEnd-handEnd
	if neck < 20 {
		t.Fatalf("HTTP neck %.2f < 20", neck)
	}
	if lower < 10 {
		t.Fatalf("HTTP lower shaft %.2f < 10", lower)
	}
	crossCentre := func(tt float64) float64 {
		lo, hi := math.Inf(1), math.Inf(-1)
		for n := -30.; n <= 30; n += .25 {
			if contactInside(colour, cx+ux*tt+uy*n, cy+uy*tt-ux*n) {
				lo = math.Min(lo, n)
				hi = math.Max(hi, n)
			}
		}
		return (lo + hi) / 2
	}
	upperT, lowerT := (headEnd+handStart)/2, (handEnd+shaftEnd)/2
	delta := math.Atan2(crossCentre(lowerT)-crossCentre(upperT), lowerT-upperT) * 180 / math.Pi
	if math.Abs(delta) > 2 {
		t.Fatalf("upper and lower shaft diverge %.3f degrees", delta)
	}
	normalWidth := func(mask *image.RGBA) float64 {
		lo, hi := math.Inf(1), math.Inf(-1)
		for y := 0; y < 420; y++ {
			for x := 0; x < 420; x++ {
				if contactInside(mask, float64(x)+.5, float64(y)+.5) {
					n := (float64(x)+.5-cx)*uy - (float64(y)+.5-cy)*ux
					lo = math.Min(lo, n)
					hi = math.Max(hi, n)
				}
			}
		}
		return hi - lo
	}
	// Compare the full palm cross-section to the shaft. The front finger pad is
	// intentionally narrower than the whole fist and has its own contour.
	fistBody, ok := contactFind(joined, "fist-body")
	if !ok || len(fistBody.Paths) != 1 {
		t.Fatal("HTTP SVG needs a separately measurable full fist body")
	}
	palm := contactMask(t, fistBody.Paths)
	palmWidth := 0.
	palmT := (handStart + handEnd) / 2
	for n := -80.; n <= 80; n += .25 {
		if contactInside(palm, cx+ux*palmT+uy*n, cy+uy*palmT-ux*n) {
			palmWidth += .25
		}
	}
	ratio := palmWidth / normalWidth(colour)
	if ratio < 1.5 || ratio > 1.8 {
		t.Fatalf("HTTP fist/shaft colour width ratio %.3f", ratio)
	}
	first, last := math.Inf(1), math.Inf(-1)
	for tt := handStart; tt <= handEnd; tt += .25 {
		x, y := cx+ux*tt, cy+uy*tt
		all, any := true, false
		for n := -24.; n <= 24; n += .25 {
			px, py := x+uy*n, y-ux*n
			if contactInside(grip, px, py) {
				any = true
				if !contactInside(flipper, px, py) {
					all = false
				}
			}
		}
		if any && all {
			first = math.Min(first, tt)
			last = math.Max(last, tt)
		}
	}
	if last-first < 8 {
		t.Fatalf("mitten never covers complete shaft width: %.2f", last-first)
	}
	shared := contactMask(t, joined.Paths[1:2])
	if !contactInside(shared, 285, 350) || !contactInside(shared, cx+ux*15, cy+uy*15) {
		t.Fatal("HTTP arm and fist are not a continuous filled silhouette")
	}
	// Render the HTTP-delivered racket with its actual resolved palette at 420px.
	palette := map[string]string{}
	for _, v := range strings.Split(root.Style, ";") {
		k, v, ok := strings.Cut(v, ":")
		if ok {
			palette[strings.TrimSpace(k)] = strings.TrimSpace(v)
		}
	}
	variable := regexp.MustCompile(`var\((--[a-z-]+),\s*(#[0-9a-fA-F]{6})\)`)
	resolve := func(fill string) string {
		return variable.ReplaceAllStringFunc(fill, func(s string) string {
			p := variable.FindStringSubmatch(s)
			if v := palette[p[1]]; v != "" {
				return v
			}
			return p[2]
		})
	}
	var paint func(contactGroup) string
	paint = func(g contactGroup) string {
		var s strings.Builder
		for _, p := range g.Paths {
			fmt.Fprintf(&s, `<path fill="%s" fill-rule="evenodd" d="%s"/>`, resolve(p.Fill), p.D)
		}
		for _, c := range g.Groups {
			s.WriteString(paint(c))
		}
		return s.String()
	}
	rendered := contactRaster(t, `<svg viewBox="0 0 420 420">`+paint(racket)+`</svg>`)
	hex := strings.TrimPrefix(resolve(hand.Paths[1].Fill), "#")
	n, err := strconv.ParseUint(hex, 16, 24)
	if err != nil {
		t.Fatal(err)
	}
	want := color.RGBA{uint8(n >> 16), uint8(n >> 8), uint8(n), 255}

	hit := false
	for tt := first + 1; tt < last-1; tt += .5 {
		x, y := cx+ux*tt, cy+uy*tt
		safe := true
		for dx := -2; dx <= 2; dx++ {
			for dy := -2; dy <= 2; dy++ {
				px, py := x+float64(dx), y+float64(dy)
				safe = safe && contactInside(fur, px, py) && !contactInside(fold, px, py)
			}
		}
		if safe {
			got := rendered.RGBAAt(int(x), int(y))
			if !bytes.Equal([]byte{got.R, got.G, got.B, got.A}, []byte{want.R, want.G, want.B, want.A}) {
				t.Fatalf("front mitten pixel %.1f,%.1f = %v, want %v", x, y, got, want)
			}
			hit = true
			break
		}
	}
	if !hit {
		t.Fatal("no mitten-colour pixel in front of shaft")
	}
	t.Logf("HTTP neck %.2f, lower %.2f, axis %.3f°, collinear deviation %.3f°, ratio %.3f, full-width overlap %.2f; 420px pixel is mitten colour", neck, lower, angle, delta, ratio, last-first)
}
