package avatar

import (
	"bytes"
	"fmt"
	"github.com/stuttgart-things/schmetterpause/web"
	"image"
	"image/gif"
	"io/fs"
	"regexp"
	"testing"
)

// Inspect actual quantised LED pixels, with separate vector masks identifying
// the two contours. A dark edge shared by the two masks would merge on black.
func TestHankEyeAndRacketStaySeparatedAt64Pixels(t *testing.T) {
	c, err := LoadCollection(web.Static, "static/img")
	if err != nil {
		t.Fatal(err)
	}
	a := DefaultForCharacter("pip", "")
	mask := func(file, group string) *image.RGBA {
		t.Helper()
		data, err := fs.ReadFile(web.Static, "static/img/pip/"+file)
		if err != nil {
			t.Fatal(err)
		}
		p := regexp.MustCompile(`(?s)<g data-pip-part="` + group + `">(.*?)</g>`).FindSubmatch(data)
		if p == nil {
			t.Fatal("missing group", group)
		}
		white := regexp.MustCompile(`fill="[^"]+"`).ReplaceAll(p[1], []byte(`fill="#ffffff"`))
		img, err := rasterise(`<svg viewBox="0 0 420 420">`+string(white)+`</svg>`, 64)
		if err != nil {
			t.Fatal(err)
		}
		return img
	}
	blade := mask("racket.svg", "blade")
	for _, face := range Faces {
		data, err := c.GIF(a, []Frame{{Face: face.Key}}, 64)
		if err != nil {
			t.Fatal(err)
		}
		img, err := gif.Decode(bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
		dark := func(x, y int) bool { r, g, b, _ := img.At(x, y).RGBA(); return r < 25*257 && g < 25*257 && b < 25*257 }
		for i := 0; i < 2; i++ {
			eye := mask("face-"+face.Key+".svg", fmt.Sprint("eye-", i))
			contacts := 0
			for y := 1; y < 63; y++ {
				for x := 1; x < 63; x++ {
					if eye.RGBAAt(x, y).R < 128 || !dark(x, y) {
						continue
					}
					for _, d := range [][2]int{{0, 0}, {1, 0}, {-1, 0}, {0, 1}, {0, -1}, {1, 1}, {1, -1}, {-1, 1}, {-1, -1}} {
						xx, yy := x+d[0], y+d[1]
						if blade.RGBAAt(xx, yy).R >= 128 && dark(xx, yy) {
							contacts++
						}
					}
				}
			}
			if contacts > 0 {
				t.Errorf("%s eye %d joins racket at %d dark pixel edges", face.Key, i, contacts)
			}
		}
	}
}
