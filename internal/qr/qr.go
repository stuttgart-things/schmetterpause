// Package qr renders a QR code as SVG geometry.
//
// It reaches neither the database nor HTTP, in line with the convention for
// logic packages in CLAUDE.md, and it returns path data rather than an image.
// A sheet pinned next to the table is printed, and a vector stays sharp at
// whatever size a printer picks; drawn inline it also costs one request less
// than a PNG would.
package qr

import (
	"fmt"
	"strings"

	"rsc.io/qr"
)

// QuietZone is the blank margin around the code, in modules. Four is what the
// specification asks for: a scanner needs it to find the edges, and a code
// printed flush against other ink is one people report as broken.
const QuietZone = 4

// level is the error correction the codes are encoded at. M recovers about
// 15% of the modules, which is the usual choice for print — enough for a
// thumbprint or a coffee ring on an office wall, without the size that H
// would add.
const level = qr.M

// Code is a rendered QR code.
type Code struct {
	// Path is SVG path data on a grid of one unit per module, with the quiet
	// zone already added to every coordinate.
	Path string
	// Size is the side length in modules, quiet zone included. It is the
	// viewBox: "0 0 Size Size".
	Size int
}

// Render encodes text as a QR code.
//
// The only realistic failure is a payload too long for the format, which for
// a URL means a very long host name.
func Render(text string) (Code, error) {
	code, err := qr.Encode(text, level)
	if err != nil {
		return Code{}, fmt.Errorf("encode %d characters as a QR code: %w", len(text), err)
	}

	var path strings.Builder
	for y := range code.Size {
		for x := 0; x < code.Size; {
			if !code.Black(x, y) {
				x++
				continue
			}

			// One rectangle per horizontal run rather than one per module:
			// the same picture in roughly half the bytes.
			run := 1
			for x+run < code.Size && code.Black(x+run, y) {
				run++
			}
			fmt.Fprintf(&path, "M%d %dh%dv1h-%dz", x+QuietZone, y+QuietZone, run, run)
			x += run
		}
	}

	return Code{Path: path.String(), Size: code.Size + 2*QuietZone}, nil
}
