package avatar

import (
	"fmt"
	"io/fs"
	"path"
)

// The empty key is sthings so every existing row and omitted form field keeps
// the original figure (ADR-0031).
type Character struct {
	Key, Name              string
	Fur, Headdress, Handle string
}

var Characters = []Character{
	{"", "sthings", "#f89828", "#f89828", "#f89828"},
	{"pogo", "Pogo", "#fff7f4", "#db37cf", "#ffb438"},
	{"pip", "Pip", "#233c87", "#d742c5", "#efa31b"},
	{"vex", "Vex", "#817d93", "#dd359a", "#f4a62b"},
	{"kiki", "Kiki", "#fcf8f5", "#c83bda", "#ef8fc0"},
}

func character(key string) (Character, bool) {
	for _, c := range Characters {
		if c.Key == key {
			return c, true
		}
	}
	return Character{}, false
}

// DefaultForCharacter selects the drawing's own materials and the player's
// derived blade. New figures use an empty fur key for their original colour.
func DefaultForCharacter(key, playerID string) Avatar {
	a := Default(playerID)
	if c, ok := character(key); ok && c.Key != "" {
		a.Character = c.Key
		a.Fur = ""
	}
	return a
}

// Collection holds complete, independently drawn kits. A new figure cannot
// accidentally inherit the old mascot's face or jewellery.
type Collection struct{ kits map[string]*Kit }

func LoadCollection(fsys fs.FS, dir string) (*Collection, error) {
	c := &Collection{kits: map[string]*Kit{}}
	for _, ch := range Characters {
		folder := ch.Key
		if folder == "" {
			folder = "sthings"
		}
		k, err := load(fsys, path.Join(dir, folder), ch.Key == "")
		if err != nil {
			return nil, fmt.Errorf("load character %s: %w", ch.Name, err)
		}
		k.character = ch
		c.kits[ch.Key] = k
	}
	return c, nil
}

func MustLoadCollection(fsys fs.FS, dir string) *Collection {
	c, err := LoadCollection(fsys, dir)
	if err != nil {
		panic(err)
	}
	return c
}

func (c *Collection) kit(a Avatar) *Kit {
	if k, ok := c.kits[a.Character]; ok {
		return k
	}
	return c.kits[""]
}
func (c *Collection) Render(a Avatar) string                  { return c.kit(a).Render(a) }
func (c *Collection) RenderFace(a Avatar, face string) string { return c.kit(a).RenderFace(a, face) }
func (c *Collection) RenderLive(a Avatar, face string) string { return c.kit(a).RenderLive(a, face) }
func (c *Collection) GIF(a Avatar, frames []Frame, size int) ([]byte, error) {
	return c.kit(a).GIF(a, frames, size)
}

// RenderFlat resolves the selected character's materials for raster previews.
func (c *Collection) RenderFlat(a Avatar, face string) string { return c.kit(a).RenderFlat(a, face) }
