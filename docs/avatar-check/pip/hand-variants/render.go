// Render isolated variant collections; never replace embedded application assets.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	"image/png"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing/fstest"

	"github.com/srwiley/oksvg"
	"github.com/srwiley/rasterx"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"

	"github.com/stuttgart-things/schmetterpause/internal/avatar"
	"github.com/stuttgart-things/schmetterpause/web"
)

const out = "docs/avatar-check/pip/hand-variants"

var navy = color.RGBA{10, 10, 18, 255}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
func collection(variant bool) *avatar.Collection {
	files := fstest.MapFS{}
	must(fs.WalkDir(web.Static, "static/img", func(name string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		data, err := exec.Command("git", "show", "1298bfe:web/"+name).Output()
		if err != nil {
			return err
		}
		if variant && name == "static/img/pip/racket.svg" {
			data, err = os.ReadFile(filepath.Join(out, "B-racket.svg"))
			if err != nil {
				return err
			}
		}
		files[name] = &fstest.MapFile{Data: data}
		return nil
	}))
	c, err := avatar.LoadCollection(files, "static/img")
	must(err)
	return c
}
func raster(svg string, size int, x, y, span float64, bg color.RGBA) *image.RGBA {
	icon, err := oksvg.ReadIconStream(strings.NewReader(svg), oksvg.StrictErrorMode)
	must(err)
	scale := float64(size) / span
	icon.SetTarget(-x*scale, -y*scale, 420*scale, 420*scale)
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	draw.Draw(img, img.Bounds(), image.NewUniform(bg), image.Point{}, draw.Src)
	icon.Draw(rasterx.NewDasher(size, size, rasterx.NewScannerGV(size, size, img, img.Bounds())), 1)
	return img
}
func canvas(w, h int, bg color.RGBA) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(img, img.Bounds(), image.NewUniform(bg), image.Point{}, draw.Src)
	return img
}
func label(img *image.RGBA, x, y int, s string) {
	d := font.Drawer{Dst: img, Src: image.White, Face: basicfont.Face7x13, Dot: fixed.P(x, y)}
	d.DrawString(s)
}
func save(name string, img image.Image) {
	f, e := os.Create(filepath.Join(out, name))
	must(e)
	if err := png.Encode(f, img); err != nil {
		must(f.Close())
		must(err)
	}
	must(f.Close())
}
func main() {
	before, b := collection(false), collection(true)
	entries := []struct {
		name, key string
		c         *avatar.Collection
	}{{"sthings", "", before}, {"1298bfe", "pip", before}, {"B80 - Andruecken", "pip", b}}
	sheet := canvas(3*208, 6*232+28, navy)
	for col, e := range entries {
		label(sheet, col*208+8, 18, e.name)
		for row := 0; row < 6; row++ {
			face := []string{"neutral", "happy", "angry"}[row%3]
			a := avatar.DefaultForCharacter(e.key, "")
			a.Blade = "light-blue"
			ctx := avatar.RenderContext{}
			name := face + " / ohne Initiale"
			if row >= 3 {
				a.Bling = "initial"
				ctx.DisplayName = "Anna"
				name = face + " / Initiale A"
			}
			svg := e.c.RenderFlat(a, face, ctx)
			img := raster(svg, 192, 0, 0, 420, navy)
			draw.Draw(sheet, image.Rect(col*208+8, row*232+28, col*208+200, row*232+220), img, image.Point{}, draw.Src)
			label(sheet, col*208+8, row*232+236, name)
		}
	}
	save("comparison-192-navy.png", sheet)
	detail := canvas(3*420, 448, navy)
	for col, e := range entries {
		a := avatar.DefaultForCharacter(e.key, "")
		a.Blade = "light-blue"
		a.Handle = "red"
		svg := e.c.RenderFlat(a, "neutral")
		img := raster(svg, 420, 235, 300, 135, navy)
		save(fmt.Sprintf("detail-%d-420.png", col), img)
		draw.Draw(detail, image.Rect(col*420, 28, col*420+420, 448), img, image.Point{}, draw.Src)
		label(detail, col*420+8, 18, e.name)
	}
	save("hand-racket-comparison-420.png", detail)
	a := avatar.DefaultForCharacter("pip", "")
	a.Blade = "light-blue"
	neutral, err := b.GIF(a, []avatar.Frame{{Face: "neutral"}}, 64)
	must(err)
	must(os.WriteFile(filepath.Join(out, "B-neutral-64.gif"), neutral, 0644))
	led, err := gif.Decode(bytes.NewReader(neutral))
	must(err)
	black := canvas(64, 64, color.RGBA{0, 0, 0, 255})
	draw.Draw(black, black.Bounds(), led, image.Point{}, draw.Over)
	save("B-neutral-64-black.png", black)
	images := map[string]image.Image{}
	for _, face := range avatar.Faces {
		raw, err := b.GIF(a, []avatar.Frame{{Face: face.Key}}, 64)
		must(err)
		images[face.Key], err = gif.Decode(bytes.NewReader(raw))
		must(err)
		must(os.WriteFile(filepath.Join(out, "B-"+face.Key+".svg"), []byte(b.RenderFlat(a, face.Key)), 0644))
	}
	minsep := 4096
	pair := ""
	for i, one := range avatar.Faces {
		for _, two := range avatar.Faces[i+1:] {
			changed := 0
			for y := 0; y < 64; y++ {
				for x := 0; x < 64; x++ {
					l := grayLevel(images[one.Key].At(x, y))
					r := grayLevel(images[two.Key].At(x, y))
					if l != r {
						changed++
					}
				}
			}
			if changed < minsep {
				minsep = changed
				pair = one.Key + "/" + two.Key
			}
		}
	}
	data, err := json.MarshalIndent(map[string]any{"variant": "B", "baseline": "1298bfe", "minimum_separation": minsep, "closest_pair": pair, "method": "64px quantized GIFs; gray/32 changed pixels, all 15 face pairs", "status": "selected product B80"}, "", "  ")
	must(err)
	must(os.WriteFile(filepath.Join(out, "separation.json"), append(data, '\n'), 0644))
	fmt.Printf("B: minimum separation %d, pair %s\n", minsep, pair)
}

func grayLevel(c color.Color) uint8 {
	gray, ok := color.GrayModel.Convert(c).(color.Gray)
	if !ok {
		panic("gray conversion returned a non-gray color")
	}
	return gray.Y / 32
}
