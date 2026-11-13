// avatar_review exports pixel-size expression and material comparison sheets.
// Run from the repository root: go run ./scripts/avatar_review.go -out /tmp/vex-review
package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	"image/png"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing/fstest"

	"github.com/srwiley/oksvg"
	"github.com/srwiley/rasterx"
	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
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
	framing := flag.Bool("framing-only", false, "export framing metrics, selection and emblem checks")
	style := flag.Bool("sthings-style-only", false, "export paired sthings/hank reference")
	cheek := flag.Bool("cheek-contour-only", false, "export hank contour before/after review")
	key := flag.String("character", "vex", "character key")
	out := flag.String("out", "docs/avatar-check/vex", "output directory")
	emblems := flag.Bool("emblems-only", false, "export hank emblem acceptance matrix")
	accessories := flag.Bool("accessories-only", false, "export only the blecki accessory acceptance matrix")
	flag.Parse()
	if *framing {
		if err := framingReview(*out); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if *style {
		if err := sthingsStyleReview(*out); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if *cheek {
		if err := cheekContourReview(*out); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if *emblems {
		if err := emblemReview(*out); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
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

// Preserve the actual pre-removal GIFs, including their former quantisation,
// without retaining obsolete cheek vectors or production logic.
func previousPipFrames() (map[string]image.Image, error) {
	frames := map[string]image.Image{}
	for _, face := range avatar.Faces {
		data, err := os.ReadFile(filepath.Join("docs/avatar-check/pip/before-removal-7969bdf", "pip-"+face.Key+".gif"))
		if err != nil {
			return nil, err
		}
		frames[face.Key], err = gif.Decode(bytes.NewReader(data))
		if err != nil {
			return nil, err
		}
	}
	return frames, nil
}

var comparisonBefore map[string]image.Image
var eyesDetail bool
var reviewNames map[string]string

func templateName(key string) string {
	for _, ch := range avatar.Characters {
		if ch.Key == key {
			return ch.Name
		}
	}
	return key
}

func run(key, out string) error {
	labelKey := key
	for _, character := range avatar.Characters {
		if character.Key == key {
			labelKey = character.Name
		}
	}
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

	if key == "pip" {
		comparisonBefore, err = previousPipFrames()
		if err != nil {
			return err
		}
		var samples []sample
		for _, face := range avatar.Faces {
			samples = append(samples, sample{face.Name + " / Alt", a, face.Key}, sample{face.Name + " / Neu", a, face.Key})
		}
		if err = sheet(collection, samples, 6, 64, color.RGBA{0, 0, 0, 255}, templateName("pip")+" | Alt / Neu je Ausdruck | 64 px schwarz", filepath.Join(out, "eyes-before-after-64-black.png")); err != nil {
			return err
		}
		comparisonBefore = nil

		var rackets []sample
		for _, blade := range []string{"light-blue", "steel", "pink"} {
			b := a
			b.Blade = blade
			label := map[string]string{"light-blue": "Hellblau", "steel": "Stahl", "pink": "Pink"}[blade]
			rackets = append(rackets, sample{label, b, "neutral"})
		}
		for _, size := range []int{192, 64} {
			if err = sheet(collection, rackets, 3, size, color.RGBA{0, 0, 0, 255}, templateName("pip")+" | Schlägerfarben", filepath.Join(out, fmt.Sprintf("racket-colours-%d-black.png", size))); err != nil {
				return err
			}
		}

		var eyes []sample
		for _, face := range avatar.Faces {
			eyes = append(eyes, sample{face.Name, a, face.Key})
		}
		eyesDetail = true
		err = sheet(collection, eyes, 3, 192, color.RGBA{10, 10, 18, 255}, templateName("pip")+" | Augenpartie | 192 px Detail", filepath.Join(out, "eyes-detail-192.png"))
		eyesDetail = false
		if err != nil {
			return err
		}
	}
	backgrounds := []struct {
		name   string
		colour color.RGBA
	}{{"app", color.RGBA{10, 10, 18, 255}}, {"black", color.RGBA{0, 0, 0, 255}}}
	palette := append([]avatar.Colour{{Key: "", Name: "Original"}}, avatar.FurPalette(key)...)
	if key == "pip" {
		var row []sample
		for _, f := range avatar.Faces {
			row = append(row, sample{f.Name, a, f.Key})
		}
		for _, size := range []int{64, 28} {
			if err = sheet(collection, row, 6, size, color.RGBA{0, 0, 0, 255}, templateName("pip")+" | original | black", filepath.Join(out, fmt.Sprintf("expressions-row-%d-black.png", size))); err != nil {
				return err
			}
		}
	}
	var expressions []sample
	for _, coat := range palette {
		for _, f := range avatar.Faces {
			b := a
			b.Fur = coat.Key
			expressions = append(expressions, sample{coat.Name + " / " + f.Name, b, f.Key})
		}
	}
	if key == "pip" {
		var critical []sample
		for _, coat := range palette {
			for _, head := range []string{"ivory", "silver"} {
				b := a
				b.Fur, b.Headdress = coat.Key, head
				if coat.Key == "grey" && head == "silver" {
					continue
				}
				critical = append(critical, sample{coat.Name + " / Stirnband " + map[string]string{"ivory": "Elfenbein", "silver": "Silber"}[head], b, "neutral"})
			}
		}
		for _, grip := range []string{"blue", "wood-dark"} {
			b := a
			b.Handle = grip
			critical = append(critical, sample{"Original / Griff " + map[string]string{"blue": "Blau", "wood-dark": "Holz dunkel"}[grip], b, "neutral"})
		}
		for _, pair := range [][2]string{{"", "blue"}, {"blue-grey", "blue"}, {"grey", "steel"}} {
			b := a
			b.Fur, b.Blade = pair[0], pair[1]
			coat := pair[0]
			if coat == "" {
				coat = "Original"
			}
			if coat == "grey" {
				coat = "Grau"
			}
			if coat == "blue-grey" {
				coat = "Blaugrau"
			}
			critical = append(critical, sample{coat + " / Schläger " + map[string]string{"blue": "Blau", "steel": "Stahl"}[pair[1]], b, "neutral"})
		}
		if err = sheet(collection, critical, 4, 64, color.RGBA{0, 0, 0, 255}, templateName("pip")+" | kritische Kombinationen | 64 px", filepath.Join(out, "critical-64-black.png")); err != nil {
			return err
		}
	}

	for _, bg := range backgrounds {
		for _, size := range []int{28, 64, 192} {
			name := fmt.Sprintf("expressions-%d-%s.png", size, bg.name)
			if err = sheet(collection, expressions, 6, size, bg.colour, fmt.Sprintf("%s | %d px | %s", labelKey, size, bg.name), filepath.Join(out, name)); err != nil {
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
		if err = sheet(collection, colours, len(palette), 144, bg.colour, labelKey+" | body alternatives | "+bg.name, filepath.Join(out, "colours-"+bg.name+".png")); err != nil {
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
			if err = sheet(collection, jewellery, len(avatar.BlingPalette(key))*2, size, bg.colour, labelKey+" | jewellery | "+bg.name, filepath.Join(out, fmt.Sprintf("jewellery-%d-%s.png", size, bg.name))); err != nil {
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
		if err = sheet(collection, grips, len(avatar.Handles), 192, bg.colour, labelKey+" | grips | "+bg.name, filepath.Join(out, "grips-"+bg.name+".png")); err != nil {
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
		if err = sheet(collection, contrasts, len(avatar.Blades), 96, bg.colour, labelKey+" | fixed colour contrast | "+bg.name, filepath.Join(out, "jewellery-blades-"+bg.name+".png")); err != nil {
			return err
		}

		if key == "pip" {
			var headbands []sample
			for _, coat := range palette {
				for _, head := range avatar.HeaddressPalette(key, coat.Key) {
					b := a
					b.Fur, b.Headdress = coat.Key, head.Key
					headbands = append(headbands, sample{coat.Name + " / " + head.Name, b, "neutral"})
				}
			}
			if err = sheet(collection, headbands, len(avatar.HeaddressPalette(key)), 144, bg.colour, labelKey+" | headbands | "+bg.name, filepath.Join(out, "headbands-"+bg.name+".png")); err != nil {
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
		if err = sheet(collection, materials, len(avatar.Handles), 112, bg.colour, labelKey+" | handles / blades | "+bg.name, filepath.Join(out, "materials-"+bg.name+".png")); err != nil {
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
		if err = sheet(collection, combinations, 2, size, bg.colour, labelKey+" | combinations | "+bg.name, filepath.Join(out, "combinations-"+bg.name+".png")); err != nil {
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
	if key == "pip" {
		note = "# hank acceptance review\n\nRegenerate: `go run ./scripts/avatar_review.go -character pip -out docs/avatar-check/pip`\n\nOpen index.html for expressions, materials and GIFs. The expressions-row PNGs show the six faces in original colours at 64 and 28 pixels on black. Full sheets include Original, Grau and Blaugrau on navy and black. hank has no cheeks; Schmuck choices are Kein Schmuck and Initiale. Grey excludes Silver; native Pink is labelled Original. The eyes-before-after-64-black.png sheet compares archived 7969bdf GIFs against hank without cheeks, each Alt/Neu pair adjacent. The eyes-detail-192.png sheet shows eyes and brows. Eyes and brows use the actual scaled sthings artwork, with white eyeballs, attached pupils and their original glints. The raised racket deliberately overlaps the cheek. Physical LED-panel contrast remains to be checked.\n"
	}
	if err = os.WriteFile(filepath.Join(out, "README.md"), []byte(note), 0644); err != nil {
		return err
	}
	fmt.Println("Review exported to", out)
	return nil
}

// Go Regular is embedded in x/image and includes German umlauts; no host font dependency.
var reviewFont = func() font.Face {
	f, err := opentype.Parse(goregular.TTF)
	if err != nil {
		panic(err)
	}
	face, err := opentype.NewFace(f, &opentype.FaceOptions{Size: 12, DPI: 72, Hinting: font.HintingFull})
	if err != nil {
		panic(err)
	}
	return face
}()

func sheet(c *avatar.Collection, samples []sample, cols, size int, bg color.RGBA, title, path string) error {
	cellW := max(size+16, 144)
	maxLines := 1
	for _, sample := range samples {
		maxLines = max(maxLines, len(strings.Split(sample.label, " / ")))
	}
	cellH := size + max(48, 20+14*maxLines)
	rows := (len(samples) + cols - 1) / cols
	canvas := image.NewRGBA(image.Rect(0, 0, cols*cellW, rows*cellH+28))
	draw.Draw(canvas, canvas.Bounds(), image.NewUniform(bg), image.Point{}, draw.Src)
	label := func(x, y int, s string) {
		d := font.Drawer{Dst: canvas, Src: image.NewUniform(color.RGBA{235, 235, 245, 255}), Face: reviewFont, Dot: fixed.P(x, y)}
		d.DrawString(s)
	}
	label(8, 18, title)
	for i, s := range samples {
		renderer := c
		ctx := avatar.RenderContext{DisplayName: reviewNames[s.label]}
		svg := renderer.RenderFlat(s.a, s.face, ctx)

		if eyesDetail {
			data, err := fs.ReadFile(web.Static, "static/img/pip/face-"+s.face+".svg")
			if err != nil {
				return err
			}
			svg = regexp.MustCompile(`(?s)<g data-pip-part="beak">.*?</g>`).ReplaceAllString(string(data), "")
			svg = strings.Replace(svg, `<g class="sthings-face"`, `<path fill="#fff6f3" d="M0 0H420V420H0Z"/><g class="sthings-face"`, 1)
		}
		icon, err := oksvg.ReadIconStream(strings.NewReader(svg), oksvg.StrictErrorMode)
		if err != nil {
			return err
		}
		icon.SetTarget(0, 0, float64(size), float64(size))

		if eyesDetail {
			scale := float64(size) / 240
			icon.SetTarget(-90*scale, -130*scale+(float64(size)-160*scale)/2, 420*scale, 420*scale)
		}
		frame := image.NewRGBA(image.Rect(0, 0, size, size))
		draw.Draw(frame, frame.Bounds(), image.NewUniform(bg), image.Point{}, draw.Src)
		icon.Draw(rasterx.NewDasher(size, size, rasterx.NewScannerGV(size, size, frame, frame.Bounds())), 1)
		x := (i%cols)*cellW + (cellW-size)/2
		y := 28 + (i/cols)*cellH
		if bg.R == 0 && bg.G == 0 && bg.B == 0 && (size == 64 || (s.a.Character == "pip" && size == 192 && !eyesDetail)) {
			data, err := renderer.GIF(s.a, []avatar.Frame{{Face: s.face}}, size, ctx)
			if err != nil {
				return err
			}
			led, err := gif.Decode(bytes.NewReader(data))
			if err != nil {
				return err
			}
			if comparisonBefore != nil && strings.HasSuffix(s.label, " / Alt") {
				led = comparisonBefore[s.face]
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
			if err = sheet(c, balls, 7, size, bg.colour, fmt.Sprintf(templateName("vex")+" | ball / blade colours | %d px | %s", size, bg.name), filepath.Join(out, ballName)); err != nil {
				return err
			}
			name := fmt.Sprintf("accessories-%d-%s.png", size, bg.name)
			if err = sheet(c, samples, len(avatar.BlingPalette(key))*2, size, bg.colour, fmt.Sprintf(templateName("vex")+" | accessories | %d px | %s", size, bg.name), filepath.Join(out, name)); err != nil {
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

func handBeforeCollection(basePath ...string) (*avatar.Collection, error) {
	files := fstest.MapFS{}
	err := fs.WalkDir(web.Static, "static/img", func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		data, err := fs.ReadFile(web.Static, name)
		if err != nil {
			return err
		}
		files[name] = &fstest.MapFile{Data: data}
		return nil
	})
	if err != nil {
		return nil, err
	}
	name := "docs/avatar-check/pip/emblems/hand-before-base.svg"
	if len(basePath) > 0 {
		name = basePath[0]
	}
	before, err := os.ReadFile(name)
	if err != nil {
		return nil, err
	}
	files["static/img/pip/base.svg"] = &fstest.MapFile{Data: before}
	return avatar.LoadCollection(files, "static/img")
}

func emblemReview(out string) error {
	c, err := avatar.LoadCollection(web.Static, "static/img")
	if err != nil {
		return err
	}
	if err = os.MkdirAll(out, 0755); err != nil {
		return err
	}
	reviewNames = map[string]string{}
	coats := []struct{ key, name string }{{"", "Original"}, {"grey", "Grau"}, {"blue-grey", "Blaugrau"}}
	heads := []struct{ key, name string }{{"fur", "Original"}, {"gold", "Gold"}, {"ruby", "Rubin"}, {"ivory", "Elfenbein"}, {"silver", "Silber"}}
	var matrix []sample
	for _, coat := range coats {
		for _, head := range heads {
			for _, bling := range avatar.HankBlings {
				for _, face := range []string{"neutral", "angry"} {
					a := avatar.DefaultForCharacter("pip", "")
					a.Fur = coat.key
					a.Headdress = head.key
					a.Blade = "light-blue"
					a.Bling = bling.Key
					faceLabel := map[string]string{"neutral": "Neutral", "angry": "Wütend"}[face]
					label := coat.name + " / " + head.name + " / " + bling.Name + " / " + faceLabel
					if coat.key == "grey" && head.key == "silver" {
						label = coat.name + " / Silber→Orig.* / " + bling.Name + " / " + faceLabel
					}
					reviewNames[label] = "Anna"
					matrix = append(matrix, sample{label, a, face})
				}
			}
		}
	}
	var letters []sample
	for _, r := range "ABCDEFGHIJKLMNOPQRSTUVWXYZÄÖÜ" {
		a := avatar.DefaultForCharacter("pip", "")
		a.Blade = "light-blue"
		a.Bling = "initial"
		label := string(r)
		reviewNames[label] = label
		letters = append(letters, sample{label, a, "neutral"})
	}
	a := avatar.DefaultForCharacter("pip", "")
	a.Blade = "light-blue"
	a.Bling = "initial"
	letters = append(letters, sample{"Schläger (kein Name)", a, "neutral"})
	for _, bg := range []struct {
		name   string
		colour color.RGBA
	}{{"app", color.RGBA{10, 10, 18, 255}}, {"black", color.RGBA{0, 0, 0, 255}}} {
		for _, size := range []int{192, 64, 28} {
			if err = sheet(c, matrix, 4, size, bg.colour, fmt.Sprintf(templateName("pip")+" | Embleme | %d px | %s", size, bg.name), filepath.Join(out, fmt.Sprintf("emblems-%d-%s.png", size, bg.name))); err != nil {
				return err
			}
			if err = sheet(c, letters, 6, size, bg.colour, fmt.Sprintf(templateName("pip")+" | A–Z Ä Ö Ü und Rückfall | %d px", size), filepath.Join(out, fmt.Sprintf("letters-%d-%s.png", size, bg.name))); err != nil {
				return err
			}
		}
	}
	comparison := []sample{letters[14], letters[16], letters[len(letters)-1]}
	for _, size := range []int{192, 64} {
		if err = sheet(c, comparison, 3, size, color.RGBA{0, 0, 0, 255}, templateName("pip")+" | O / Q / Rückfall", filepath.Join(out, fmt.Sprintf("fallback-oq-%d-black.png", size))); err != nil {
			return err
		}
	}
	reviewNames = nil
	before, err := handBeforeCollection()
	if err != nil {
		return err
	}
	for _, name := range []string{"hand-before-420.png", "hand-after-420.png"} {
		plain := avatar.DefaultForCharacter("pip", "")
		plain.Blade = "light-blue"
		plain.Handle = "red"
		renderer := c
		if strings.Contains(name, "before") {
			renderer = before
		}
		svg := renderer.RenderFlat(plain, "neutral")
		icon, err := oksvg.ReadIconStream(strings.NewReader(svg), oksvg.StrictErrorMode)
		if err != nil {
			return err
		}
		scale := 420.0 / 130.0
		icon.SetTarget(-225*scale, -310*scale, 420*scale, 420*scale)
		frame := image.NewRGBA(image.Rect(0, 0, 420, 420))
		draw.Draw(frame, frame.Bounds(), image.Black, image.Point{}, draw.Src)
		icon.Draw(rasterx.NewDasher(420, 420, rasterx.NewScannerGV(420, 420, frame, frame.Bounds())), 1)
		f, err := os.Create(filepath.Join(out, name))
		if err != nil {
			return err
		}
		err = png.Encode(f, frame)
		closeErr := f.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}

// cheekContourReview preserves the previous base and renders the exact native area.
func cheekContourReview(out string) error {
	if err := os.MkdirAll(out, 0755); err != nil {
		return err
	}
	current, err := avatar.LoadCollection(web.Static, "static/img")
	if err != nil {
		return err
	}
	before, err := handBeforeCollection("docs/avatar-check/pip/cheek-contour/before-base.svg")
	if err != nil {
		return err
	}
	a := avatar.DefaultForCharacter("pip", "")
	a.Blade = "light-blue"
	for _, entry := range []struct {
		name string
		c    *avatar.Collection
	}{{"before", before}, {"after", current}} {
		svg := entry.c.RenderFlat(a, "neutral")
		icon, err := oksvg.ReadIconStream(strings.NewReader(svg), oksvg.StrictErrorMode)
		if err != nil {
			return err
		}
		scale := 420.0 / 100.0
		icon.SetTarget(-285*scale, -175*scale, 420*scale, 420*scale)
		frame := image.NewRGBA(image.Rect(0, 0, 420, 420))
		draw.Draw(frame, frame.Bounds(), image.Black, image.Point{}, draw.Src)
		icon.Draw(rasterx.NewDasher(420, 420, rasterx.NewScannerGV(420, 420, frame, frame.Bounds())), 1)
		f, err := os.Create(filepath.Join(out, "cheek-"+entry.name+"-420.png"))
		if err != nil {
			return err
		}
		err = png.Encode(f, frame)
		closeErr := f.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
	}
	var expressions []sample
	for _, face := range avatar.Faces {
		expressions = append(expressions, sample{face.Name, a, face.Key})
	}
	return sheet(current, expressions, 6, 192, color.RGBA{0, 0, 0, 255}, templateName("pip")+" | bereinigte Wangenkontur", filepath.Join(out, "expressions-192-black.png"))
}

func sthingsStyleReview(out string) error {
	c, err := avatar.LoadCollection(web.Static, "static/img")
	if err != nil {
		return err
	}
	if err = os.MkdirAll(out, 0755); err != nil {
		return err
	}
	var pairs []sample
	for _, face := range avatar.Faces {
		for _, key := range []string{"", "pip"} {
			a := avatar.DefaultForCharacter(key, "")
			a.Blade = "light-blue"
			pairs = append(pairs, sample{templateName(key) + " / " + face.Name, a, face.Key})
		}
	}
	return sheet(c, pairs, 2, 192, color.RGBA{10, 10, 18, 255}, "sthings / hank | sechs Ausdrücke", filepath.Join(out, "sthings-hank-192-app.png"))
}

func framingReview(out string) error {
	if err := sthingsStyleReview(out); err != nil {
		return err
	}
	raw, err := os.ReadFile(filepath.Join(out, "framing-metadata.json"))
	if err != nil {
		return err
	}
	metrics := map[string]map[string]interface{}{}
	if err = json.Unmarshal(raw, &metrics); err != nil {
		return err
	}
	for _, name := range []string{"sthings", "blecki", "hank-before", "hank-after"} {
		data, err := os.ReadFile(filepath.Join(out, name+"-head-mask.svg"))
		if err != nil {
			return err
		}
		icon, err := oksvg.ReadIconStream(bytes.NewReader(data), oksvg.StrictErrorMode)
		if err != nil {
			return err
		}
		icon.SetTarget(0, 0, 420, 420)
		img := image.NewRGBA(image.Rect(0, 0, 420, 420))
		draw.Draw(img, img.Bounds(), image.Black, image.Point{}, draw.Src)
		icon.Draw(rasterx.NewDasher(420, 420, rasterx.NewScannerGV(420, 420, img, img.Bounds())), 1)
		minX, minY, maxX, maxY := 420, 420, 0, 0
		for y := 0; y < 420; y++ {
			for x := 0; x < 420; x++ {
				if img.RGBAAt(x, y).R >= 128 {
					minX = min(minX, x)
					minY = min(minY, y)
					maxX = max(maxX, x+1)
					maxY = max(maxY, y+1)
				}
			}
		}
		metrics[name]["head_width_pixels"] = maxX - minX
		metrics[name]["head_width_percent"] = float64(maxX-minX) / 4.2
		metrics[name]["top_margin_pixels"] = minY
		metrics[name]["head_left_margin"] = minX
		metrics[name]["head_right_margin"] = 420 - maxX
		f, err := os.Create(filepath.Join(out, name+"-head-mask.png"))
		if err != nil {
			return err
		}
		err = png.Encode(f, img)
		f.Close()
		if err != nil {
			return err
		}
	}
	result, err := json.MarshalIndent(metrics, "", "  ")
	if err != nil {
		return err
	}
	if err = os.WriteFile(filepath.Join(out, "framing-measurements.json"), append(result, '\n'), 0644); err != nil {
		return err
	}
	c, err := avatar.LoadCollection(web.Static, "static/img")
	if err != nil {
		return err
	}
	var choices []sample
	for _, ch := range avatar.Characters {
		a := avatar.DefaultForCharacter(ch.Key, "")
		a.Blade = "light-blue"
		choices = append(choices, sample{ch.Name, a, "neutral"})
	}
	if err = sheet(c, choices, 3, 60, color.RGBA{10, 10, 18, 255}, "Figurenauswahl | 60 px", filepath.Join(out, "selection-60-app.png")); err != nil {
		return err
	}
	reviewNames = map[string]string{}
	var emblems []sample
	for _, face := range []string{"neutral", "angry"} {
		for _, initial := range []string{"A", ""} {
			a := avatar.DefaultForCharacter("pip", "")
			a.Bling = "initial"
			a.Blade = "light-blue"
			name := "Initiale A"
			if initial == "" {
				name = "Rückfall"
			}
			label := name + " / " + map[string]string{"neutral": "Neutral", "angry": "Wütend"}[face]
			reviewNames[label] = initial
			emblems = append(emblems, sample{label, a, face})
		}
	}
	for _, size := range []int{192, 64} {
		for _, bg := range []struct {
			name string
			c    color.RGBA
		}{{"app", color.RGBA{10, 10, 18, 255}}, {"black", color.RGBA{0, 0, 0, 255}}} {
			if err = sheet(c, emblems, 2, size, bg.c, "hank | Emblem im neuen Ausschnitt", filepath.Join(out, fmt.Sprintf("emblems-%d-%s.png", size, bg.name))); err != nil {
				return err
			}
		}
	}
	reviewNames = nil
	return nil
}
