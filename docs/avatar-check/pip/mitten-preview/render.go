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

	"github.com/stuttgart-things/schmetterpause/internal/avatar"
	"github.com/stuttgart-things/schmetterpause/web"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

const out = "docs/avatar-check/pip/mitten-preview"

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
		data, err := exec.Command("git", "show", "a99eac2:web/"+name).Output()
		if err != nil {
			return err
		}
		if variant && (name == "static/img/pip/racket.svg" || name == "static/img/pip/base.svg") {
			part := strings.TrimSuffix(filepath.Base(name), ".svg")
			data, err = os.ReadFile(filepath.Join(out, "mitten-"+part+".svg"))
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
	baseline, candidate := collection(false), collection(true)
	entries := []struct {
		name, key string
		c         *avatar.Collection
	}{{"sthings", "", baseline}, {"hank a99eac2", "pip", baseline}, {"hank Faeustling - Produkt", "pip", candidate}}
	detail := canvas(1260, 448, navy)
	for i, e := range entries {
		a := avatar.DefaultForCharacter(e.key, "")
		a.Blade = "light-blue"
		a.Handle = "red"
		img := raster(e.c.RenderFlat(a, "neutral"), 420, 235, 275, 150, navy)
		save(fmt.Sprintf("hand-%d-420.png", i), img)
		draw.Draw(detail, image.Rect(i*420, 28, i*420+420, 448), img, image.Point{}, draw.Src)
		label(detail, i*420+8, 18, e.name)
	}
	save("hand-comparison-420.png", detail)
	sheet := canvas(6*208, 244, navy)
	a := avatar.DefaultForCharacter("pip", "")
	a.Blade = "light-blue"
	images := map[string]image.Image{}
	var liveHTML strings.Builder
	liveHTML.WriteString(`<!doctype html><html lang="de"><meta charset="utf-8"><link rel="stylesheet" href="app-render.css"><style>body{margin:0;padding:0;background:#0a0a12;color:#eee;font:14px system-ui;max-width:none}#live-sheet{display:flex;width:1248px;background:#0a0a12}.face{width:208px;flex:none;padding:8px;box-sizing:border-box}.figure{width:192px;height:192px}.figure svg.sthings{display:block;width:192px;height:192px}.face h2{font:14px system-ui;margin:4px 0}</style><div id="live-sheet">`)
	css, err := fs.ReadFile(web.Static, "static/css/app.css")
	must(err)
	must(os.WriteFile(filepath.Join(out, "app-render.css"), css, 0644))
	for i, face := range avatar.Faces {
		// Same production collection methods used by the app, with proposal
		// base/racket assets only in this isolated in-memory collection.
		flat := candidate.RenderFlat(a, face.Key)
		fmt.Fprintf(&liveHTML, `<section class="face"><h2>%s</h2><div class="figure" id="face-%s">%s</div></section>`, face.Name, face.Key, candidate.RenderLive(a, face.Key))
		must(os.WriteFile(filepath.Join(out, "mitten-"+face.Key+".svg"), []byte(flat), 0644))
		must(os.WriteFile(filepath.Join(out, "mitten-live-"+face.Key+".svg"), []byte(candidate.RenderLive(a, face.Key)), 0644))
		img := raster(flat, 192, 0, 0, 420, navy)
		save("expression-"+face.Key+"-192.png", img)
		draw.Draw(sheet, image.Rect(i*208+8, 28, i*208+200, 220), img, image.Point{}, draw.Src)
		label(sheet, i*208+8, 18, "App-Renderer / Produkt")
		label(sheet, i*208+8, 238, face.Name)
		raw, err := candidate.GIF(a, []avatar.Frame{{Face: face.Key}}, 64)
		must(err)
		images[face.Key], err = gif.Decode(bytes.NewReader(raw))
		must(err)
	}
	save("expressions-192-navy.png", sheet)
	liveHTML.WriteString(`</div></html>`)
	must(os.WriteFile(filepath.Join(out, "live-expressions.html"), []byte(liveHTML.String()), 0644))
	minimum := 4096
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
			if changed < minimum {
				minimum = changed
				pair = one.Key + "/" + two.Key
			}
		}
	}
	data, err := json.MarshalIndent(map[string]any{"renderer": "production avatar.Collection.RenderFlat / RenderLive / GIF", "asset_loading": "a99eac2 with mitten assets overridden only in memory", "minimum_separation_64": minimum, "closest_pair": pair, "approved_product": true}, "", "  ")
	must(err)
	must(os.WriteFile(filepath.Join(out, "renderer-verification.json"), append(data, '\n'), 0644))
	fmt.Println(string(data))
}

func grayLevel(c color.Color) uint8 {
	gray, ok := color.GrayModel.Convert(c).(color.Gray)
	if !ok {
		panic("gray conversion returned a non-gray color")
	}
	return gray.Y / 32
}
