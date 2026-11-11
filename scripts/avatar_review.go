// avatar_review exports pixel-size expression and material comparison sheets.
// Run from the repository root: go run ./scripts/avatar_review.go -out /tmp/vex-review
package main

import (
	"bytes"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	"image/png"
	"os"
	"path/filepath"
	"strings"

	"github.com/srwiley/oksvg"
	"github.com/srwiley/rasterx"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"

	"github.com/stuttgart-things/schmetterpause/internal/avatar"
	"github.com/stuttgart-things/schmetterpause/web"
)

type sample struct {
	label string
	a     avatar.Avatar
	face  string
}

func main() {
	key := flag.String("character", "vex", "character key")
	out := flag.String("out", "docs/avatar-check/vex", "output directory")
	flag.Parse()
	if err := run(*key, *out); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run(key, out string) error {
	known := false
	for _, c := range avatar.Characters {
		if c.Key == key {
			known = true
		}
	}
	if !known {
		return fmt.Errorf("unknown character %q", key)
	}
	collection, err := avatar.LoadCollection(web.Static, "static/img")
	if err != nil {
		return err
	}
	if err = os.MkdirAll(out, 0755); err != nil {
		return err
	}
	a := avatar.DefaultForCharacter(key, "")
	a.Blade = "light-blue"
	backgrounds := []struct {
		name   string
		colour color.RGBA
	}{{"app", color.RGBA{10, 10, 18, 255}}, {"black", color.RGBA{0, 0, 0, 255}}}
	palette := append([]avatar.Colour{{Key: "", Name: "original"}}, avatar.FurPalette(key)...)
	var expressions []sample
	for _, coat := range palette {
		for _, f := range avatar.Faces {
			b := a
			b.Fur = coat.Key
			expressions = append(expressions, sample{coat.Name + " / " + f.Key, b, f.Key})
		}
	}
	for _, bg := range backgrounds {
		for _, size := range []int{28, 64, 192} {
			name := fmt.Sprintf("expressions-%d-%s.png", size, bg.name)
			if err = sheet(collection, expressions, 6, size, bg.colour, fmt.Sprintf("%s | %d px | %s", key, size, bg.name), filepath.Join(out, name)); err != nil {
				return err
			}
		}
		var colours []sample
		for _, f := range []string{"neutral", "angry"} {
			for _, c := range palette {
				b := a
				b.Fur = c.Key
				label := c.Key + " [review]"
				if c.Key == "" {
					label = "original"
				}
				colours = append(colours, sample{label + " / " + f, b, f})
			}
		}
		if err = sheet(collection, colours, len(palette), 144, bg.colour, key+" | body alternatives | "+bg.name, filepath.Join(out, "colours-"+bg.name+".png")); err != nil {
			return err
		}
		var jewellery []sample
		for _, coat := range palette {
			for _, bling := range avatar.BlingPalette(key) {
				for _, face := range []string{"neutral", "angry"} {
					b := a
					b.Fur = coat.Key
					b.Bling = bling.Key
					label := bling.Key
					if label == "" {
						label = "none"
					}
					jewellery = append(jewellery, sample{coat.Name + " / " + label + " / " + face, b, face})
				}
			}
		}
		for _, size := range []int{28, 64, 192} {
			if err = sheet(collection, jewellery, 10, size, bg.colour, key+" | jewellery | "+bg.name, filepath.Join(out, fmt.Sprintf("jewellery-%d-%s.png", size, bg.name))); err != nil {
				return err
			}
		}
		var grips []sample
		for _, coat := range palette {
			for _, handle := range avatar.Handles {
				b := a
				b.Fur = coat.Key
				b.Handle = handle.Key
				grips = append(grips, sample{coat.Name + " / " + handle.Name, b, "neutral"})
			}
		}
		if err = sheet(collection, grips, len(avatar.Handles), 192, bg.colour, key+" | grips | "+bg.name, filepath.Join(out, "grips-"+bg.name+".png")); err != nil {
			return err
		}
		// Check fixed jewellery against every paddle colour as well as every coat.
		var contrasts []sample
		for _, coat := range palette {
			for _, bling := range avatar.BlingPalette(key)[1:] {
				for _, blade := range avatar.Blades {
					b := a
					b.Fur = coat.Key
					b.Bling = bling.Key
					b.Blade = blade.Key
					contrasts = append(contrasts, sample{coat.Name + " / " + bling.Key + " / " + blade.Key, b, "neutral"})
				}
			}
		}
		if err = sheet(collection, contrasts, len(avatar.Blades), 96, bg.colour, key+" | fixed colour contrast | "+bg.name, filepath.Join(out, "jewellery-blades-"+bg.name+".png")); err != nil {
			return err
		}
		var materials []sample
		for _, c := range avatar.Handles {
			b := a
			b.Handle = c.Key
			label := c.Key
			if c.Key == avatar.SameAsFur {
				label = "original"
			}
			materials = append(materials, sample{"handle: " + label, b, "neutral"})
		}
		for _, c := range avatar.Blades {
			b := a
			b.Blade = c.Key
			materials = append(materials, sample{"blade: " + c.Key, b, "neutral"})
		}
		if err = sheet(collection, materials, len(avatar.Handles), 112, bg.colour, key+" | handles / blades | "+bg.name, filepath.Join(out, "materials-"+bg.name+".png")); err != nil {
			return err
		}
		combinations := []sample{}
		b := a
		b.Blade = "steel"
		combinations = append(combinations, sample{"steel / original coat", b, "neutral"})
		b = a
		b.Handle = "ivory"
		combinations = append(combinations, sample{"ivory / pale paw", b, "neutral"})
		b = a
		b.Handle = "wood-dark"
		b.Blade = "moss"
		combinations = append(combinations, sample{"moss / dark wood", b, "neutral"})
		b = a
		b.Handle = "wood-dark"
		b.Blade = "petrol"
		combinations = append(combinations, sample{"petrol / dark wood", b, "neutral"})
		size := 144
		if bg.name == "black" {
			size = 64
		}
		if err = sheet(collection, combinations, 2, size, bg.colour, key+" | combinations | "+bg.name, filepath.Join(out, "combinations-"+bg.name+".png")); err != nil {
			return err
		}

	}
	for _, f := range avatar.Faces {
		if err = os.WriteFile(filepath.Join(out, key+"-"+f.Key+".svg"), []byte(collection.RenderFlat(a, f.Key)), 0644); err != nil {
			return err
		}
	}
	frames := make([]avatar.Frame, 0, len(avatar.Faces))
	for _, f := range avatar.Faces {
		frames = append(frames, avatar.Frame{Face: f.Key, Delay: 130})
		data, err := collection.GIF(a, []avatar.Frame{{Face: f.Key}}, 64)
		if err != nil {
			return err
		}
		if err = os.WriteFile(filepath.Join(out, key+"-"+f.Key+".gif"), data, 0644); err != nil {
			return err
		}
	}
	for _, loop := range []struct {
		name   string
		frames []avatar.Frame
	}{{"all-faces", frames}, {"cheer", avatar.Cheer}} {
		for _, size := range []int{64, 256} {
			data, err := collection.GIF(a, loop.frames, size)
			if err != nil {
				return err
			}
			name := key + "-" + loop.name
			if size != 64 {
				name += "-256"
			}
			if err = os.WriteFile(filepath.Join(out, name+".gif"), data, 0644); err != nil {
				return err
			}
		}
	}

	note := `# Blecki acceptance review

Regenerate from the repository root:

sh
go run ./scripts/avatar_review.go -character vex -out docs/avatar-check/vex


The storage key and folder remain vex; the public name is Blecki.

- Expressions: all six faces, all four coats, at native 192, 64 and 28 px.
- Jewellery: all five slot options, neutral and angry, all four coats,
  at native 192, 64 and 28 px.
- Grips: all seven handle choices, all four coats, at 192 px.
- Jewellery/blades: all four pieces against every coat and blade colour.
- Every sheet is exported on navy (#0a0a12) and pure black.
- Black 64 px sheets use the actual GIF palette quantisation.
- Individual SVGs/GIFs, the production cheer loop and a six-face review loop
  are included. The review loop does not change game-controlled expressions.

## Observations and remaining review

The enlarged grip distinguishes red, dark wood and ivory at profile size.
All fixed jewellery colours retain a filled black outline across the four
coats. No palette colours were silently changed. The blade head coordinates
remain identical to the accepted workshop revision.

At 28 px the moon and tooth reduce to approximately one or two light pixels;
their shapes cannot be reliably identified. The captain C is not readable at
that size either. They are rendered consistently at every size as requested,
without special hiding or substitutions. Their small-size design needs user
review. The bandana remains a more distinct red accent.

Physical LED contrast is still unverified: these are software previews,
not photographs of a real panel. In particular, dark wood and moss on black
should be checked on the device. No runtime hides jewellery automatically.
`
	if err = os.WriteFile(filepath.Join(out, "README.md"), []byte(note), 0644); err != nil {
		return err
	}
	fmt.Println("Review exported to", out)
	return nil
}
func sheet(c *avatar.Collection, samples []sample, cols, size int, bg color.RGBA, title, path string) error {
	cellW := max(size+16, 144)
	cellH := size + 48
	rows := (len(samples) + cols - 1) / cols
	canvas := image.NewRGBA(image.Rect(0, 0, cols*cellW, rows*cellH+28))
	draw.Draw(canvas, canvas.Bounds(), image.NewUniform(bg), image.Point{}, draw.Src)
	label := func(x, y int, s string) {
		d := font.Drawer{Dst: canvas, Src: image.NewUniform(color.RGBA{235, 235, 245, 255}), Face: basicfont.Face7x13, Dot: fixed.P(x, y)}
		d.DrawString(s)
	}
	label(8, 18, title)
	for i, s := range samples {
		icon, err := oksvg.ReadIconStream(strings.NewReader(c.RenderFlat(s.a, s.face)), oksvg.StrictErrorMode)
		if err != nil {
			return err
		}
		icon.SetTarget(0, 0, float64(size), float64(size))
		frame := image.NewRGBA(image.Rect(0, 0, size, size))
		draw.Draw(frame, frame.Bounds(), image.NewUniform(bg), image.Point{}, draw.Src)
		icon.Draw(rasterx.NewDasher(size, size, rasterx.NewScannerGV(size, size, frame, frame.Bounds())), 1)
		x := (i%cols)*cellW + (cellW-size)/2
		y := 28 + (i/cols)*cellH
		if bg.R == 0 && bg.G == 0 && bg.B == 0 && size == 64 {
			data, err := c.GIF(s.a, []avatar.Frame{{Face: s.face}}, 64)
			if err != nil {
				return err
			}
			led, err := gif.Decode(bytes.NewReader(data))
			if err != nil {
				return err
			}
			draw.Draw(frame, frame.Bounds(), led, image.Point{}, draw.Src)
		}
		draw.Draw(canvas, image.Rect(x, y, x+size, y+size), frame, image.Point{}, draw.Src)
		for line, text := range strings.Split(s.label, " / ") {
			label((i%cols)*cellW+8, y+size+20+line*14, text)
		}
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	err = png.Encode(f, canvas)
	closeErr := f.Close()
	if err != nil {
		return err
	}
	return closeErr
}
