package main

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing/fstest"

	"github.com/srwiley/oksvg"
	"github.com/srwiley/rasterx"
	"github.com/stuttgart-things/schmetterpause/internal/avatar"
	"github.com/stuttgart-things/schmetterpause/web"
)

const out = "docs/avatar-check/pip/full-fist-preview"

func must(e error) {
	if e != nil {
		panic(e)
	}
}
func collection(proposal bool) *avatar.Collection {
	m := fstest.MapFS{}
	must(fs.WalkDir(web.Static, "static/img", func(n string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if d.IsDir() {
			return nil
		}
		b, e := fs.ReadFile(web.Static, n)
		if e != nil {
			return e
		}
		if proposal && (n == "static/img/pip/base.svg" || n == "static/img/pip/racket.svg") {
			p := strings.TrimSuffix(filepath.Base(n), ".svg")
			b, e = os.ReadFile(filepath.Join(out, "proposal-"+p+".svg"))
			if e != nil {
				return e
			}
		}
		m[n] = &fstest.MapFile{Data: b}
		return nil
	}))
	c, e := avatar.LoadCollection(m, "static/img")
	must(e)
	return c
}
func render(c *avatar.Collection, key string, size int) *image.RGBA {
	a := avatar.DefaultForCharacter(key, "")
	a.Blade = "light-blue"
	a.Handle = "red"
	svg := c.RenderFlat(a, "neutral")
	ic, e := oksvg.ReadIconStream(strings.NewReader(svg), oksvg.StrictErrorMode)
	must(e)
	sc := float64(size) / 150
	ic.SetTarget(-235*sc, -275*sc, 420*sc, 420*sc)
	im := image.NewRGBA(image.Rect(0, 0, size, size))
	draw.Draw(im, im.Bounds(), image.NewUniform(color.RGBA{10, 10, 18, 255}), image.Point{}, draw.Src)
	ic.Draw(rasterx.NewDasher(size, size, rasterx.NewScannerGV(size, size, im, im.Bounds())), 1)
	return im
}
func save(name string, im image.Image) {
	f, e := os.Create(filepath.Join(out, name))
	must(e)
	defer f.Close()
	must(png.Encode(f, im))
}
func main() {
	base := collection(false)
	next := collection(true)
	canvas := image.NewRGBA(image.Rect(0, 0, 1260, 448))
	draw.Draw(canvas, canvas.Bounds(), image.NewUniform(color.RGBA{10, 10, 18, 255}), image.Point{}, draw.Src)
	for i, x := range []struct {
		k, n string
		c    *avatar.Collection
	}{{"sthings", "sthings", base}, {"pip", "aktueller Stand", nil}, {"pip", "neue Faust", next}} {
		c := x.c
		if c == nil {
			c, _ = avatar.LoadCollection(web.Static, "static/img")
		}
		im := render(c, x.k, 420)
		save(fmt.Sprintf("hand-%d-420.png", i), im)
		draw.Draw(canvas, image.Rect(i*420, 28, i*420+420, 448), im, image.Point{}, draw.Src)
	}
	save("hand-comparison-420.png", canvas)
	faces := image.NewRGBA(image.Rect(0, 0, 6*208, 244))
	draw.Draw(faces, faces.Bounds(), image.NewUniform(color.RGBA{10, 10, 18, 255}), image.Point{}, draw.Src)
	a := avatar.DefaultForCharacter("pip", "")
	for i, f := range avatar.Faces {
		svg := next.RenderFlat(a, f.Key)
		ic, e := oksvg.ReadIconStream(strings.NewReader(svg), oksvg.StrictErrorMode)
		must(e)
		ic.SetTarget(float64(i*208+8), 28, 192, 192)
		ic.Draw(rasterx.NewDasher(1248, 244, rasterx.NewScannerGV(1248, 244, faces, faces.Bounds())), 1)
	}
	save("expressions-192.png", faces)
}
