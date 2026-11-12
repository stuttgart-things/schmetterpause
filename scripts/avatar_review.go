// avatar_review exports pixel-size expression and material comparison sheets.
// Run from the repository root: go run ./scripts/avatar_review.go -out /tmp/vex-review
package main

import (
	"bytes"
	"encoding/csv"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strconv"
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
	accessories := flag.Bool("accessories-only", false, "export only the blecki accessory acceptance matrix")
	flag.Parse()
	if *accessories {
		if err := accessoryReview(*key, *out); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
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
			if err = sheet(collection, jewellery, len(avatar.BlingPalette(key))*2, size, bg.colour, key+" | jewellery | "+bg.name, filepath.Join(out, fmt.Sprintf("jewellery-%d-%s.png", size, bg.name))); err != nil {
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

		if key == "pip" {
			var headbands []sample
			for _, coat := range palette {
				for _, head := range avatar.Headdresses {
					b := a
					b.Fur, b.Headdress = coat.Key, head.Key
					headbands = append(headbands, sample{coat.Name + " / " + head.Name, b, "neutral"})
				}
			}
			if err = sheet(collection, headbands, len(avatar.Headdresses), 144, bg.colour, key+" | headbands | "+bg.name, filepath.Join(out, "headbands-"+bg.name+".png")); err != nil {
				return err
			}
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

	note := `# blecki acceptance review

Regenerate from the repository root:

sh
go run ./scripts/avatar_review.go -character vex -out docs/avatar-check/vex


The storage key and folder remain vex; the public name is blecki.

- Expressions: all six faces, all four coats, at native 192, 64 and 28 px.
- Jewellery: both slot options (none and ball), neutral and angry, all four coats,
  at native 192, 64 and 28 px.
- Grips: all seven handle choices, all four coats, at 192 px.
- Jewellery/blades: both accessories against every coat and blade colour.
- Every sheet is exported on navy (#0a0a12) and pure black.
- Black 64 px sheets use the actual GIF palette quantisation.
- Individual SVGs/GIFs, the production cheer loop and a six-face review loop
  are included. The review loop does not change game-controlled expressions.

## Observations and remaining review

The enlarged grip distinguishes red, dark wood and ivory at profile size.
All fixed jewellery colours retain a filled black outline across the four
coats. No palette colours were silently changed. The blade head coordinates
remain identical to the accepted workshop revision.

Ball colours are white (default), orange and yellow with calculated shadows.
The ball's geometry and contour-derived anchor are unchanged. The focused
review includes all colour/blade combinations and a fill-contrast CSV.
Orange on light-blue, steel and petrol needs particular review at 28 px/LED;
the unchanged black ring helps, but does not establish physical LED contrast.

Only the table-tennis ball remains as blecki's optional accessory. Towel,
captain band and neck pieces are retired. Old keys fall back to no jewellery.
The active ball geometry and all base/face/material paths remain unchanged.

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
		svg := c.RenderFlat(s.a, s.face)
		icon, err := oksvg.ReadIconStream(strings.NewReader(svg), oksvg.StrictErrorMode)
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

// accessoryReview exports the focused placement matrix without refreshing
// older face/material acceptance artefacts from another review.
func accessoryReview(key, out string) error {
	if key != "vex" {
		return fmt.Errorf("accessory placement review requires blecki")
	}
	c, err := avatar.LoadCollection(web.Static, "static/img")
	if err != nil {
		return err
	}
	if err = os.MkdirAll(out, 0755); err != nil {
		return err
	}
	palette := append([]avatar.Colour{{Key: "", Name: "Original"}}, avatar.FurPalette(key)...)
	var samples []sample
	for _, coat := range palette {
		for _, bling := range avatar.BlingPalette(key) {
			for _, face := range []string{"neutral", "angry"} {
				a := avatar.DefaultForCharacter(key, "")
				a.Fur = coat.Key
				a.Bling = bling.Key
				a.Blade = "light-blue"
				label := bling.Key
				if label == "" {
					label = "none"
				}
				fur := coat.Key
				if fur == "" {
					fur = "original"
				}
				samples = append(samples, sample{fur + " / " + label + " / " + face, a, face})
			}
		}
	}
	for _, bg := range []struct {
		name   string
		colour color.RGBA
	}{{"app", color.RGBA{10, 10, 18, 255}}, {"black", color.RGBA{0, 0, 0, 255}}} {
		var balls []sample
		for _, coat := range palette {
			for _, face := range []string{"neutral", "angry"} {
				for _, tone := range avatar.BallColors {
					for _, blade := range avatar.Blades {
						a := avatar.DefaultForCharacter(key, "")
						a.Fur = coat.Key
						a.Bling = "ball"
						a.BallColor = tone.Key
						a.Blade = blade.Key
						fur := coat.Key
						if fur == "" {
							fur = "original"
						}
						toneName := tone.Key
						if toneName == "" {
							toneName = "white"
						}
						balls = append(balls, sample{fur + " " + toneName + " / " + blade.Key + " / " + face, a, face})
					}
				}
			}
		}
		for _, size := range []int{192, 64, 28} {
			ballName := fmt.Sprintf("ball-blades-%d-%s.png", size, bg.name)
			if err = sheet(c, balls, 7, size, bg.colour, fmt.Sprintf("blecki | ball / blade colours | %d px | %s", size, bg.name), filepath.Join(out, ballName)); err != nil {
				return err
			}
			name := fmt.Sprintf("accessories-%d-%s.png", size, bg.name)
			if err = sheet(c, samples, len(avatar.BlingPalette(key))*2, size, bg.colour, fmt.Sprintf("blecki | accessories | %d px | %s", size, bg.name), filepath.Join(out, name)); err != nil {
				return err
			}
		}
	}
	if err = writeBallContrast(out); err != nil {
		return err
	}
	fmt.Println("Accessory matrix exported to", out)
	return nil
}

// Fill-only ratios are a diagnostic; the unchanged black outline adds separation.
func writeBallContrast(out string) error {
	var data bytes.Buffer
	var err error
	w := csv.NewWriter(&data)
	if err = w.Write([]string{"ball", "blade", "fill_luminance_ratio", "review"}); err != nil {
		return err
	}
	luminance := func(hex string) (float64, error) {
		n, err := strconv.ParseUint(strings.TrimPrefix(hex, "#"), 16, 32)
		if err != nil {
			return 0, err
		}
		values := []float64{float64((n>>16)&255) / 255, float64((n>>8)&255) / 255, float64(n&255) / 255}
		for i, v := range values {
			if v <= .04045 {
				values[i] = v / 12.92
			} else {
				values[i] = math.Pow((v+.055)/1.055, 2.4)
			}
		}
		return .2126*values[0] + .7152*values[1] + .0722*values[2], nil
	}
	for _, ball := range avatar.BallColors {
		for _, blade := range avatar.Blades {
			a, err := luminance(ball.Hex)
			if err != nil {
				return err
			}
			b, err := luminance(blade.Hex)
			if err != nil {
				return err
			}
			ratio := (max(a, b) + .05) / (min(a, b) + .05)
			note := "outline retained"
			if ratio < 1.5 {
				note = "low fill contrast: inspect 28px and real LED"
			} else if ratio < 2.1 {
				note = "inspect small-size outline"
			}
			key := ball.Key
			if key == "" {
				key = "white"
			}
			if err = w.Write([]string{key, blade.Key, fmt.Sprintf("%.2f", ratio), note}); err != nil {
				return err
			}
		}
	}
	w.Flush()
	if err = w.Error(); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(out, "ball-colour-contrast.csv"), data.Bytes(), 0644)
}
