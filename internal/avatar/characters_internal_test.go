package avatar

import (
	"fmt"
	"image"
	"image/color"
	"strings"
	"testing"

	"github.com/stuttgart-things/schmetterpause/web"
)

// Exercise every complete kit through the same flat SVG used by the LED GIF.
func TestCharacterFacesAtDisplaySizes(t *testing.T) {
	collection, err := LoadCollection(web.Static, "static/img")
	if err != nil {
		t.Fatal(err)
	}
	for _, ch := range Characters {
		t.Run(ch.Name, func(t *testing.T) {
			k := collection.kits[ch.Key]
			a := DefaultForCharacter(ch.Key, "")

			for _, size := range []int{28, 40, 64, 80, 420} {
				t.Run(fmt.Sprint(size), func(t *testing.T) {
					images := map[string]*image.RGBA{}
					for _, face := range Faces {
						svg := k.RenderFlat(a, face.Key)
						if strings.Contains(svg, "var(") {
							t.Fatal("unresolved colour")
						}
						img, err := rasterise(svg, size)
						if err != nil {
							t.Fatalf("%s: %v", face.Key, err)
						}
						images[face.Key] = img
					}
					if size != 40 && size != 64 {
						return
					}
					// Compare quantised luminance, ignoring tiny antialiasing differences.
					// All six expressions must retain distinct shapes on a monochrome panel.
					for i, a := range Faces {
						for _, b := range Faces[i+1:] {
							changed := 0
							for y := 0; y < size; y++ {
								for x := 0; x < size; x++ {
									aGray := color.GrayModel.Convert(images[a.Key].At(x, y)).(color.Gray).Y / 32
									bGray := color.GrayModel.Convert(images[b.Key].At(x, y)).(color.Gray).Y / 32
									if aGray != bGray {
										changed++
									}
								}
							}
							if changed < 6 {
								t.Errorf("%s and %s differ in only %d monochrome pixels", a.Key, b.Key, changed)
							}
						}
					}
				})
			}

		})
	}
}

func TestTemplateNamesAreLowercase(t *testing.T) {
	for _, ch := range Characters {
		if ch.Name != strings.ToLower(ch.Name) {
			t.Errorf("template %q is not lowercase", ch.Name)
		}
	}
	for _, ch := range characterCatalogue(Character{Name: "Hank"}, Character{Name: "BLECKI"}, Character{Name: "SThings"}) {
		if ch.Name != strings.ToLower(ch.Name) {
			t.Errorf("catalogue failed to normalize %q", ch.Name)
		}
	}
}
