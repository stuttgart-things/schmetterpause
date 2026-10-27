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
	base   string
	faces  map[string]string
	blings map[string]string
}

// Load reads every part the option lists name from dir in fsys. A face or a
// piece of bling without its file is an error here, at startup, rather than
// a figure with a hole in it on somebody's profile.
func Load(fsys fs.FS, dir string) (*Kit, error) {
	read := func(name string) (string, error) {
		b, err := fs.ReadFile(fsys, path.Join(dir, name+".svg"))
		if err != nil {
			return "", fmt.Errorf("read avatar part %s: %w", name, err)
		}
		return inner(string(b))
	}
	k := &Kit{faces: map[string]string{}, blings: map[string]string{}}
	var err error
	if k.base, err = read("base"); err != nil {
		return nil, err
	}
	for _, f := range Faces {
		if k.faces[f.Key], err = read("face-" + f.Key); err != nil {
			return nil, err
		}
	}
	for _, b := range Blings {
		if b.Key == NoBling {
			continue
		}
		if k.blings[b.Key], err = read("bling-" + b.Key); err != nil {
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
	return k.render(a, "", k.faces[Faces[0].Key])
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
// twice on one page is harmless.
//
// An unknown face shows the neutral one.
func (k *Kit) RenderLive(a Avatar, face string) string {
	if !hasOption(Faces, face) {
		face = Faces[0].Key
	}
	var b strings.Builder
	b.WriteString(`<style>.sthings[data-face] .sthings-face{display:none}`)
	for _, f := range Faces {
		fmt.Fprintf(&b, `.sthings[data-face="%[1]s"] .sthings-face[data-face="%[1]s"]{display:inline}`, f.Key)
	}
	b.WriteString(`</style>`)
	for _, f := range Faces {
		b.WriteString(k.faces[f.Key])
	}
	return k.render(a, face, b.String())
}

func (k *Kit) render(a Avatar, face, faces string) string {
	a = Fill(a, "")
	fur, _ := findColour(Furs, a.Fur)
	head, _ := findColour(Headdresses, a.Headdress)
	handle, _ := findColour(Handles, a.Handle)

	var style strings.Builder
	prop := func(name, value string) {
		fmt.Fprintf(&style, "--%s:%s;", name, value)
	}
	prop("fur", fur.Hex)
	prop("fur-dark", fur.Shade)
	prop("headdress", orFur(head.Hex, fur.Hex))
	prop("headdress-dark", orFur(head.Shade, fur.Shade))
	prop("handle", orFur(handle.Hex, fur.Hex))
	prop("handle-dark", orFur(handle.Shade, fur.Shade))
	if blade, ok := findColour(Blades, a.Blade); ok {
		prop("paddle", blade.Hex)
	}

	var b strings.Builder
	b.WriteString(`<svg viewBox="0 0 420 420" xmlns="http://www.w3.org/2000/svg" aria-hidden="true" class="sthings"`)
	if face != "" {
		fmt.Fprintf(&b, ` data-face="%s"`, face)
	}
	b.WriteString(` style="`)
	b.WriteString(strings.TrimSuffix(style.String(), ";"))
	b.WriteString(`">`)
	b.WriteString(k.base)
	b.WriteString(faces)
	b.WriteString(k.blings[a.Bling])
	b.WriteString(`</svg>`)
	return b.String()
}

func orFur(hex, fur string) string {
	if hex == "" {
		return fur
	}
	return hex
}
