package avatar

import (
	"errors"
	"image"
	"io/fs"
	"math/rand/v2"
	"reflect"
	"strings"
	"testing"

	"github.com/stuttgart-things/schmetterpause/web"
)

func TestVexPaletteIsAppliedToStoredSubmittedAndRandomFigures(t *testing.T) {
	var got []string
	for _, c := range FurPalette("vex") {
		got = append(got, c.Key)
	}
	if !reflect.DeepEqual(got, []string{"sand", "white", "chocolate"}) {
		t.Fatalf("Vex palette: %v", got)
	}
	for _, key := range []string{"orange", "ginger", "cinnamon", "gold", "cream"} {
		a := DefaultForCharacter("vex", "anna")
		a.Fur = key
		if !errors.Is(Validate(a), ErrUnknown) {
			t.Errorf("Vex accepts forbidden %s", key)
		}
		if filled := Fill(a, "anna"); filled.Fur != "" || filled.Character != "vex" {
			t.Errorf("stored %s did not fall back to Vex's original coat: %+v", key, filled)
		}
		for _, ch := range Characters {
			if ch.Key == "vex" {
				continue
			}
			a = DefaultForCharacter(ch.Key, "anna")
			a.Fur = key
			if err := Validate(a); err != nil {
				t.Errorf("%s lost its %s option: %v", ch.Name, key, err)
			}
		}
	}
	rng := rand.New(rand.NewPCG(125, 328))
	vexes := 0
	for range 2000 {
		a := Random(rng)
		if err := Validate(a); err != nil {
			t.Fatal(err)
		}
		if a.Character == "vex" {
			vexes++
			if !validFur("vex", a.Fur) {
				t.Errorf("random Vex has %s", a.Fur)
			}
		}
	}
	if vexes == 0 {
		t.Fatal("random never selected Vex")
	}
}

func TestVexBaseContainsNoEyeLidOrMouthRemnants(t *testing.T) {
	b, err := fs.ReadFile(web.Static, "static/img/vex/base.svg")
	if err != nil {
		t.Fatal(err)
	}
	svg := varRef.ReplaceAllString(string(b), "$2")
	img, err := rasterise(svg, 420)
	if err != nil {
		t.Fatal(err)
	}
	// These interior planes contain fur/muzzle only. The permanent nose is
	// outside both regions; ears, silhouette and jaw outline are outside too.
	for _, r := range []image.Rectangle{image.Rect(146, 164, 317, 233), image.Rect(189, 244, 234, 282)} {
		ink := 0
		for y := r.Min.Y; y < r.Max.Y; y++ {
			for x := r.Min.X; x < r.Max.X; x++ {
				p := img.RGBAAt(x, y)
				if p.R < 16 && p.G < 16 && p.B < 16 {
					ink++
				}
			}
		}
		if ink != 0 {
			t.Errorf("base still has %d ink pixels in face interior %v", ink, r)
		}
	}
	if strings.Contains(svg, "sthings-face") {
		t.Error("a face group leaked into the base")
	}
}

func TestVexOpenEyesHaveSeparateRoundPupilsAndWhiteHighlights(t *testing.T) {
	c, err := LoadCollection(web.Static, "static/img")
	if err != nil {
		t.Fatal(err)
	}
	a := DefaultForCharacter("vex", "")
	for _, face := range []string{"surprised", "skeptical", "angry", "sad"} {
		t.Run(face, func(t *testing.T) {
			img, err := rasterise(c.RenderFlat(a, face), 420)
			if err != nil {
				t.Fatal(err)
			}
			for i, anchor := range []image.Point{{182, 201}, {285, 202}} {
				seed := image.Pt(anchor.X-3, anchor.Y+17)
				if face == "surprised" {
					seed = image.Pt(anchor.X-2, anchor.Y+5)
				}
				if face == "skeptical" && i == 0 {
					seed = image.Pt(anchor.X-3, anchor.Y+9)
				}
				if face == "sad" {
					seed = image.Pt(anchor.X, anchor.Y+21)
					if i == 1 {
						seed.Y = anchor.Y + 18
					}
				}
				dark := func(p image.Point) bool { v := img.RGBAAt(p.X, p.Y); return v.R < 24 && v.G < 24 && v.B < 24 }
				if !dark(seed) {
					t.Fatalf("eye %d has no dark pupil at %v", i, seed)
				}
				seen := map[image.Point]bool{seed: true}
				queue := []image.Point{seed}
				box := image.Rect(seed.X, seed.Y, seed.X+1, seed.Y+1)
				for len(queue) > 0 {
					p := queue[0]
					queue = queue[1:]
					box = box.Union(image.Rect(p.X, p.Y, p.X+1, p.Y+1))
					for _, d := range []image.Point{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
						n := p.Add(d)
						if n.In(img.Bounds()) && !seen[n] && dark(n) {
							seen[n] = true
							queue = append(queue, n)
						}
					}
				}
				// Larger glossy pupils are intentional; a joined lid exceeds this frame.
				if box.Dx() > 32 || box.Dy() > 32 || box.Dx() < 12 || box.Dy() < 12 {
					t.Errorf("eye %d pupil joins a black lid/outline: %v", i, box)
				}
				if float64(box.Dx())/float64(box.Dy()) < .8 || float64(box.Dx())/float64(box.Dy()) > 1.25 {
					t.Errorf("eye %d pupil is not round: %v", i, box)
				}
				highlights, whiteAround := 0, 0
				for y := box.Min.Y - 3; y < box.Max.Y+3; y++ {
					for x := box.Min.X - 3; x < box.Max.X+3; x++ {
						v := img.RGBAAt(x, y)
						if v.R > 240 && v.G > 240 && v.B > 240 {
							if image.Pt(x, y).In(box) {
								highlights++
							} else {
								whiteAround++
							}
						}
					}
				}
				if highlights < 5 || whiteAround < 12 {
					t.Errorf("eye %d lacks highlight or white sclera: %d/%d", i, highlights, whiteAround)
				}
			}
			if face != "surprised" {
				teeth := 0
				for y := 260; y < 294; y++ {
					for x := 188; x < 268; x++ {
						v := img.RGBAAt(x, y)
						if v.R > 240 && v.G > 240 && v.B > 240 {
							teeth++
						}
					}
				}
				if teeth > 0 {
					t.Errorf("negative mouth contains %d tooth-white pixels", teeth)
				}
			}
		})
	}
}

// Above the forearm, the head and paddle must remain separate silhouettes.
func TestVexPaddleClearsHead(t *testing.T) {
	c, err := LoadCollection(web.Static, "static/img")
	if err != nil {
		t.Fatal(err)
	}
	a := DefaultForCharacter("vex", "")
	img, err := rasterise(c.RenderFlat(a, "neutral"), 420)
	if err != nil {
		t.Fatal(err)
	}
	foreground := func(p image.Point) bool { v := img.RGBAAt(p.X, p.Y); return v.R >= 4 || v.G >= 4 || v.B >= 4 }
	labels := map[image.Point]int{}
	components := [][]image.Point{}
	for y := 0; y < 330; y++ {
		for x := 0; x < 420; x++ {
			seed := image.Pt(x, y)
			if labels[seed] != 0 || !foreground(seed) {
				continue
			}
			id := len(components) + 1
			labels[seed] = id
			points := []image.Point{seed}
			for i := 0; i < len(points); i++ {
				for _, d := range []image.Point{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
					n := points[i].Add(d)
					if n.X < 0 || n.X >= 420 || n.Y < 0 || n.Y >= 330 || labels[n] != 0 || !foreground(n) {
						continue
					}
					labels[n] = id
					points = append(points, n)
				}
			}
			components = append(components, points)
		}
	}
	head := labels[image.Pt(230, 100)]
	blade := labels[image.Pt(350, 300)]
	if head == 0 || blade == 0 || head == blade {
		t.Fatal("paddle touches head or is missing")
	}
	for _, p := range components[blade-1] {
		for dy := -12; dy <= 12; dy++ {
			for dx := -12; dx <= 12; dx++ {
				if dx*dx+dy*dy < 144 && labels[p.Add(image.Pt(dx, dy))] == head {
					t.Fatalf("paddle has less than 12 units clearance at %v", p)
				}
			}
		}
	}
	for y := 0; y < 420; y++ {
		if foreground(image.Pt(419, y)) {
			t.Fatal("paddle clipped at canvas edge")
		}
	}
}

func TestVexNegativeLidAnglesAreDistinct(t *testing.T) {
	c, err := LoadCollection(web.Static, "static/img")
	if err != nil {
		t.Fatal(err)
	}
	a := DefaultForCharacter("vex", "")
	for _, face := range []string{"angry", "sad", "skeptical"} {
		img, err := rasterise(c.RenderFlat(a, face), 420)
		if err != nil {
			t.Fatal(err)
		}
		top := func(x int) int {
			for y := 175; y < 230; y++ {
				p := img.RGBAAt(x, y)
				if p.R > 240 && p.G > 240 && p.B > 240 {
					return y
				}
			}
			t.Fatalf("%s: no eye white at x=%d", face, x)
			return 0
		}
		left := top(198) - top(166)
		right := top(301) - top(269)
		switch face {
		case "angry":
			if left < 5 || right > -5 {
				t.Errorf("angry lids must descend towards nose: %d/%d", left, right)
			}
		case "sad":
			if left > -5 || right < 5 {
				t.Errorf("sad lids must descend outwards: %d/%d", left, right)
			}
		case "skeptical":
			if right < -2 || right > 2 {
				t.Errorf("skeptical right lid must be horizontal: %d", right)
			}
		}
	}
}

func TestBleckiJewelleryUsesOneCharacterSpecificSlot(t *testing.T) {
	c, err := LoadCollection(web.Static, "static/img")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"", "bandana", "captain", "moon", "wolf-tooth"}
	if len(Characters) != 2 || Characters[1].Name != "Blecki" {
		t.Fatal("release must offer sthings and Blecki only")
	}
	for i, b := range BlingPalette("vex") {
		if b.Key != want[i] {
			t.Errorf("jewellery order: %s", b.Key)
		}
		a := DefaultForCharacter("vex", "anna")
		a.Bling = b.Key
		if err := Validate(a); err != nil {
			t.Fatal(err)
		}
		if Fill(a, "anna") != a {
			t.Fatal("Fill drops Blecki jewellery")
		}
		svg := c.Render(a)
		if b.Key != "" {
			at := strings.Index(svg, `data-bling="`+b.Key+`"`)
			if at < 0 || at > strings.Index(svg, `class="sthings-face"`) {
				t.Error("jewellery must precede face")
			}
			a.Character = ""
			if Validate(a) == nil || Fill(a, "anna").Bling != "" {
				t.Error("Blecki jewellery leaks to sthings")
			}
		}
	}
	seen := map[string]bool{}
	rng := rand.New(rand.NewPCG(44, 66))
	for range 3000 {
		a := Random(rng)
		if a.Character == "vex" {
			seen[a.Bling] = true
			if a.Fur == "cinnamon" {
				t.Fatal("roll chose cinnamon")
			}
		}
	}
	if len(seen) != 5 {
		t.Errorf("roll misses jewellery options: %v", seen)
	}
}

func TestBleckiGripHasVisibleColourBeyondPaw(t *testing.T) {
	c, err := LoadCollection(web.Static, "static/img")
	if err != nil {
		t.Fatal(err)
	}
	a := DefaultForCharacter("vex", "")
	a.Handle = "red"
	img, err := rasterise(c.RenderFlat(a, "neutral"), 420)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	box := image.Rectangle{}
	for y := 375; y < 420; y++ {
		for x := 200; x < 265; x++ {
			v := img.RGBAAt(x, y)
			if v.R == 200 && v.G == 40 && v.B == 40 {
				count++
				r := image.Rect(x, y, x+1, y+1)
				if box.Empty() {
					box = r
				} else {
					box = box.Union(r)
				}
			}
		}
	}
	if count < 300 || box.Dx() < 30 || box.Dy() < 18 {
		t.Errorf("exposed colour too small: %d pixels in %v", count, box)
	}
}
