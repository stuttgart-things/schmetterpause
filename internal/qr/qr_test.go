package qr_test

import (
	"regexp"
	"strconv"
	"strings"
	"testing"

	upstream "rsc.io/qr"

	"github.com/stuttgart-things/schmetterpause/internal/qr"
)

const sample = "https://schmetterpause.example.com/#match"

// rect matches one "M<x> <y>h<n>v1h-<n>z" run.
var rect = regexp.MustCompile(`M(\d+) (\d+)h(\d+)v1h-(\d+)z`)

// cells parses the rendered path back into the set of black modules it draws.
// Anything the path contains beyond those runs is a failure: the sheet embeds
// the string as-is, so unparsed leftovers would be drawn and never noticed.
func cells(t *testing.T, path string) map[[2]int]bool {
	t.Helper()

	matches := rect.FindAllStringSubmatchIndex(path, -1)

	black := make(map[[2]int]bool)
	end := 0
	for _, m := range matches {
		if m[0] != end {
			t.Fatalf("path holds %q between two runs", path[end:m[0]])
		}
		end = m[1]

		x := atoi(t, path[m[2]:m[3]])
		y := atoi(t, path[m[4]:m[5]])
		run := atoi(t, path[m[6]:m[7]])
		if back := path[m[8]:m[9]]; back != strconv.Itoa(run) {
			t.Fatalf("run at %d,%d moves right by %d and back by %s", x, y, run, back)
		}
		for i := range run {
			if black[[2]int{x + i, y}] {
				t.Fatalf("module %d,%d is drawn twice", x+i, y)
			}
			black[[2]int{x + i, y}] = true
		}
	}
	if end != len(path) {
		t.Fatalf("path has trailing %q", path[end:])
	}
	return black
}

func atoi(t *testing.T, s string) int {
	t.Helper()

	v, err := strconv.Atoi(s)
	if err != nil {
		t.Fatalf("%q is not a number: %v", s, err)
	}
	return v
}

// TestPathDrawsExactlyTheEncodedModules is the point of the package: whatever
// the encoder considers black has to end up in the path, shifted by the quiet
// zone and by nothing else. A transposed or off-by-one grid still looks like a
// QR code and simply does not scan.
func TestPathDrawsExactlyTheEncodedModules(t *testing.T) {
	code, err := qr.Render(sample)
	if err != nil {
		t.Fatalf("Render(): %v", err)
	}

	want, err := upstream.Encode(sample, upstream.M)
	if err != nil {
		t.Fatalf("Encode(): %v", err)
	}

	black := cells(t, code.Path)

	drawn := 0
	for y := range want.Size {
		for x := range want.Size {
			got := black[[2]int{x + qr.QuietZone, y + qr.QuietZone}]
			if got != want.Black(x, y) {
				t.Fatalf("module %d,%d drawn = %v, want %v", x, y, got, want.Black(x, y))
			}
			if got {
				drawn++
			}
		}
	}
	if drawn != len(black) {
		t.Errorf("the path draws %d modules, %d of them outside the code", len(black), len(black)-drawn)
	}
}

func TestSizeIncludesTheQuietZoneOnBothSides(t *testing.T) {
	code, err := qr.Render(sample)
	if err != nil {
		t.Fatalf("Render(): %v", err)
	}

	want, err := upstream.Encode(sample, upstream.M)
	if err != nil {
		t.Fatalf("Encode(): %v", err)
	}

	if code.Size != want.Size+2*qr.QuietZone {
		t.Errorf("Size = %d, want %d", code.Size, want.Size+2*qr.QuietZone)
	}

	for cell := range cells(t, code.Path) {
		for _, v := range cell {
			if v < qr.QuietZone || v >= code.Size-qr.QuietZone {
				t.Fatalf("module %v lies in the quiet zone of a %d module code", cell, code.Size)
			}
		}
	}
}

// TestFinderPatternsSitInTheCorners checks the result without asking the
// encoder what it produced. The three finders are fixed by the specification,
// so their presence at the right offsets says the grid is upright and the
// quiet zone is where it is claimed to be.
func TestFinderPatternsSitInTheCorners(t *testing.T) {
	code, err := qr.Render(sample)
	if err != nil {
		t.Fatalf("Render(): %v", err)
	}
	black := cells(t, code.Path)

	// A finder is a 7x7 block: black ring, white ring, 3x3 black core.
	finder := [7][7]bool{}
	for y := range 7 {
		for x := range 7 {
			ring := x == 0 || x == 6 || y == 0 || y == 6
			core := x >= 2 && x <= 4 && y >= 2 && y <= 4
			finder[y][x] = ring || core
		}
	}

	last := code.Size - qr.QuietZone - 7
	corners := map[string][2]int{
		"top left":     {qr.QuietZone, qr.QuietZone},
		"top right":    {last, qr.QuietZone},
		"bottom left":  {qr.QuietZone, last},
		"bottom right": {last, last}, // deliberately empty: a QR code has three finders.
	}

	for name, origin := range corners {
		matches := true
		for y := range 7 {
			for x := range 7 {
				if black[[2]int{origin[0] + x, origin[1] + y}] != finder[y][x] {
					matches = false
				}
			}
		}
		if want := name != "bottom right"; matches != want {
			t.Errorf("finder pattern %s: present = %v, want %v", name, matches, want)
		}
	}
}

func TestRenderRejectsAPayloadThatDoesNotFit(t *testing.T) {
	if _, err := qr.Render(strings.Repeat("a", 8000)); err == nil {
		t.Error("Render() accepted a payload no QR code can hold")
	}
}
