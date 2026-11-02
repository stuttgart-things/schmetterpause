package avatar

import (
	"fmt"
	"io/fs"
	"path"
	"strings"
)

// Kit is the drawn parts, read once. Render puts a figure together from it.
//
// The parts come in through an fs.FS rather than an embed of this package's
// own: they live under web/static/img/sthings, where they are also served,
// and an embed cannot reach outside its package's directory.
type Kit struct {
	// looks is every character's drawing, by key: the mascot's under
	// NoCharacter, every other animal's under its own.
	looks  map[string]look
	blings map[string]string
}

// look is one animal: the figure without a face, and its faces by key. Every
// animal has every face in Faces, so the Zählwerk can switch any of them the
// same way.
type look struct {
	base  string
	faces map[string]string
}

// Load reads every part the option lists name from dir in fsys: the mascot's
// base, faces and bling, and every other character's base and faces from the
// directory beside it named by its key (static/img/fynn next to
// static/img/sthings). A missing file is an error here, at startup, rather
// than a figure with a hole in it on somebody's profile.
func Load(fsys fs.FS, dir string) (*Kit, error) {
	read := func(dir, name string) (string, error) {
		b, err := fs.ReadFile(fsys, path.Join(dir, name+".svg"))
		if err != nil {
			return "", fmt.Errorf("read avatar part %s/%s: %w", path.Base(dir), name, err)
		}
		return inner(string(b))
	}
	readLook := func(dir string) (look, error) {
		l := look{faces: map[string]string{}}
		var err error
		if l.base, err = read(dir, "base"); err != nil {
			return look{}, err
		}
		for _, f := range Faces {
			if l.faces[f.Key], err = read(dir, "face-"+f.Key); err != nil {
				return look{}, err
			}
		}
		return l, nil
	}

	k := &Kit{looks: map[string]look{}, blings: map[string]string{}}
	for _, c := range Characters {
		d := dir
		if c.Key != NoCharacter {
			d = path.Join(path.Dir(dir), c.Key)
		}
		l, err := readLook(d)
		if err != nil {
			return nil, err
		}
		k.looks[c.Key] = l
	}
	for _, b := range Blings {
		if b.Key == NoBling {
			continue
		}
		var err error
		if k.blings[b.Key], err = read(dir, "bling-"+b.Key); err != nil {
			return nil, err
		}
	}
	return k, nil
}

// MustLoad is Load for the parts compiled into the binary, where a missing
// one is a build that should never have shipped rather than a condition to
// handle. The tests load the same parts, so it fails there first.
func MustLoad(fsys fs.FS, dir string) *Kit {
	k, err := Load(fsys, dir)
	if err != nil {
		panic(err)
	}
	return k
}

// inner is a part without its own <svg> element, so that several parts can
// share the one Render writes.
func inner(doc string) (string, error) {
	start := strings.Index(doc, "<svg")
	if start < 0 {
		return "", fmt.Errorf("avatar part has no <svg> element")
	}
	open := strings.Index(doc[start:], ">")
	end := strings.LastIndex(doc, "</svg>")
	if open < 0 || end < start+open {
		return "", fmt.Errorf("avatar part has no closed <svg> element")
	}
	return doc[start+open+1 : end], nil
}

// Render is the figure as one inline SVG element with the neutral face: the
// base, then the face, then the bling, in the order the briefing gives. It is
// the figure for a page, where nothing is happening to it.
//
// The colours go into a style attribute on the element itself, as the
// custom properties the parts read. Two reasons. The same markup, with every
// face, is what GET /avatars/{id}.svg hands to the Zählwerk, which has no
// stylesheet of ours. And every value in it comes from the palettes in this
// package, never from what somebody typed.
//
// Parts a does not name, or names with an unknown key, fall back as in Fill
// for nobody: the drawing, with its red blade.
func (k *Kit) Render(a Avatar) string {
	return k.render(a, "", Faces[0].Key)
}

// RenderLive is the figure with every face, for standing next to a running
// match. data-face on the element picks the one that shows; a client switches
// it by setting that attribute and nothing else, so a face change on every
// point costs no request.
//
// The switch is a <style> element inside the figure, which is what the
// briefing rules out for the parts and why it is here and not there: it is
// the one rule this markup needs wherever it ends up, inline in a page of ours
// or in the Zählwerk's. Its selectors are scoped to .sthings, so inlining it
// twice on one page is harmless. Every animal carries the class, so the rule
// and the attribute are the same for all of them.
//
// An unknown face shows the neutral one.
func (k *Kit) RenderLive(a Avatar, face string) string {
	if !hasOption(Faces, face) {
		face = Faces[0].Key
	}
	keys := make([]string, len(Faces))
	for i, f := range Faces {
		keys[i] = f.Key
	}
	return k.render(a, face, keys...)
}

// render writes the figure with the faces named in show, and data-face set to
// face when it is not empty. More than one face brings the rule that shows
// only the one data-face picks.
func (k *Kit) render(a Avatar, face string, show ...string) string {
	a = Fill(a, "")
	l := k.looks[a.Character]

	var b strings.Builder
	b.WriteString(`<svg viewBox="0 0 420 420" xmlns="http://www.w3.org/2000/svg" aria-hidden="true" class="sthings"`)
	if a.Character != NoCharacter {
		fmt.Fprintf(&b, ` data-character="%s"`, a.Character)
	}
	if face != "" {
		fmt.Fprintf(&b, ` data-face="%s"`, face)
	}
	if style := k.style(a); style != "" {
		fmt.Fprintf(&b, ` style="%s"`, style)
	}
	b.WriteString(`>`)
	b.WriteString(l.base)
	if len(show) > 1 {
		b.WriteString(`<style>.sthings[data-face] .sthings-face{display:none}`)
		for _, f := range show {
			fmt.Fprintf(&b, `.sthings[data-face="%[1]s"] .sthings-face[data-face="%[1]s"]{display:inline}`, f)
		}
		b.WriteString(`</style>`)
	}
	for _, f := range show {
		b.WriteString(l.faces[f])
	}
	if a.Character == NoCharacter {
		b.WriteString(k.blings[a.Bling])
	}
	b.WriteString(`</svg>`)
	return b.String()
}

// style is the custom properties a's colours set, as the value of a style
// attribute.
//
// The mascot takes every colour, and "same as the fur" is the fur's. Another
// animal has its own fur, so of a's choices only headdress, handle and blade
// reach it, and "same as the fur" leaves a surface as the animal is drawn:
// the property is not set, and the part's fallback shows.
func (k *Kit) style(a Avatar) string {
	fur, _ := findColour(Furs, a.Fur)
	head, _ := findColour(Headdresses, a.Headdress)
	handle, _ := findColour(Handles, a.Handle)
	blade, hasBlade := findColour(Blades, a.Blade)

	var props []string
	prop := func(name, value string) { props = append(props, "--"+name+":"+value) }
	if a.Character == NoCharacter {
		prop("fur", fur.Hex)
		prop("fur-dark", fur.Shade)
		prop("headdress", orFur(head.Hex, fur.Hex))
		prop("headdress-dark", orFur(head.Shade, fur.Shade))
		prop("handle", orFur(handle.Hex, fur.Hex))
		prop("handle-dark", orFur(handle.Shade, fur.Shade))
	} else {
		if head.Hex != "" {
			prop("headdress", head.Hex)
		}
		if handle.Hex != "" {
			prop("handle", handle.Hex)
		}
	}
	if hasBlade {
		prop("paddle", blade.Hex)
	}
	return strings.Join(props, ";")
}

func orFur(hex, fur string) string {
	if hex == "" {
		return fur
	}
	return hex
}
