package avatar

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	"regexp"
	"strconv"
	"strings"

	"github.com/srwiley/oksvg"
	"github.com/srwiley/rasterx"
)

// Sizes a GIF may be asked for. The LED panel it is made for is 64 pixels
// square; below 32 the face is gone, above 256 it is a picture for a screen,
// which has the SVG.
const (
	MinGIFSize     = 32
	DefaultGIFSize = 64
	MaxGIFSize     = 256
)

// Frame is one picture of an animation: a face, and how long it stands in
// hundredths of a second, the unit GIF counts in.
type Frame struct {
	Face  string
	Delay int
}

// Cheer is the loop a GIF without a face plays: the figure pleased, then
// delighted for a moment, twice, and a breath before it starts again. Made for
// the panel's moment after a won set or match (docs/adr/0030).
var Cheer = []Frame{
	{"happy", 70}, {"surprised", 25}, {"happy", 70}, {"surprised", 25}, {"neutral", 40},
}

// RenderFlat is the figure with one face and its colours written out: no
// <style>, no custom properties. Everything a rasteriser without a CSS engine
// needs, and nothing it would trip over.
//
// Render and RenderLive leave the colours to the custom properties the parts
// read, which a browser resolves and oksvg does not. Here every var(--name,
// fallback) becomes the value the figure sets, or the fallback where it sets
// none.
func (k *Kit) RenderFlat(a Avatar, face string) string {
	if !hasOption(Faces, face) {
		face = Faces[0].Key
	}
	svg := k.render(a, "", k.faces[face])

	props := map[string]string{}
	if m := styleAttr.FindStringSubmatch(svg); m != nil {
		for _, decl := range strings.Split(m[1], ";") {
			if name, value, ok := strings.Cut(decl, ":"); ok {
				props[strings.TrimPrefix(name, "--")] = value
			}
		}
	}
	svg = styleAttr.ReplaceAllLiteralString(svg, "")
	return varRef.ReplaceAllStringFunc(svg, func(ref string) string {
		m := varRef.FindStringSubmatch(ref)
		if v, ok := props[m[1]]; ok {
			return v
		}
		return m[2]
	})
}

var (
	// styleAttr is the custom properties render writes on the root, the only
	// style attribute in a figure: TestPartsFollowTheBriefing keeps the parts
	// free of them.
	styleAttr = regexp.MustCompile(`style="([^"]*)"`)
	// varRef is a custom property as the parts read one: always with a
	// fallback, always a colour.
	varRef = regexp.MustCompile(`var\(--([a-z-]+),\s*(#[0-9a-fA-F]{6})\)`)
	// hexColour is every colour a flat figure paints with.
	hexColour = regexp.MustCompile(`#[0-9a-fA-F]{6}\b`)
)

// GIF is the figure as an animated GIF of size pixels square, one picture per
// frame, looping, on black: the panel's off.
//
// The palette is the figure's own colours and nothing else, and every pixel
// takes the nearest of them without dithering. The edges a rasteriser
// smooths would otherwise become a speckle of in-between colours, which on a
// grid of LEDs reads as noise rather than as a smooth line.
func (k *Kit) GIF(a Avatar, frames []Frame, size int) ([]byte, error) {
	size = min(max(size, MinGIFSize), MaxGIFSize)
	if len(frames) == 0 {
		return nil, fmt.Errorf("a GIF needs a frame")
	}

	flats := make([]string, len(frames))
	for i, f := range frames {
		flats[i] = k.RenderFlat(a, f.Face)
	}
	pal := paletteOf(flats)

	anim := &gif.GIF{LoopCount: 0}
	for i, f := range frames {
		img, err := rasterise(flats[i], size)
		if err != nil {
			return nil, err
		}
		p := image.NewPaletted(img.Bounds(), pal)
		draw.Draw(p, p.Bounds(), img, image.Point{}, draw.Src)
		anim.Image = append(anim.Image, p)
		anim.Delay = append(anim.Delay, f.Delay)
	}

	var buf bytes.Buffer
	if err := gif.EncodeAll(&buf, anim); err != nil {
		return nil, fmt.Errorf("encode figure GIF: %w", err)
	}
	return buf.Bytes(), nil
}

// paletteOf is black and every colour the flat figures name, in the order
// they first appear. A figure names a few dozen; a GIF holds 256.
func paletteOf(flats []string) color.Palette {
	pal := color.Palette{color.Black}
	seen := map[string]bool{}
	for _, svg := range flats {
		for _, hex := range hexColour.FindAllString(svg, -1) {
			hex = strings.ToLower(hex)
			if seen[hex] || len(pal) == 256 {
				continue
			}
			seen[hex] = true
			v, _ := strconv.ParseUint(hex[1:], 16, 32)
			pal = append(pal, color.RGBA{R: uint8(v >> 16), G: uint8(v >> 8), B: uint8(v), A: 0xff})
		}
	}
	return pal
}

// rasterise draws a flat figure size pixels square on black.
func rasterise(svg string, size int) (*image.RGBA, error) {
	icon, err := oksvg.ReadIconStream(strings.NewReader(svg), oksvg.StrictErrorMode)
	if err != nil {
		return nil, fmt.Errorf("read flat figure: %w", err)
	}
	icon.SetTarget(0, 0, float64(size), float64(size))
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	draw.Draw(img, img.Bounds(), image.Black, image.Point{}, draw.Src)
	icon.Draw(rasterx.NewDasher(size, size, rasterx.NewScannerGV(size, size, img, img.Bounds())), 1)
	return img, nil
}
