package avatar

// Colour is one entry of a palette. Key is what is stored and sent by a form,
// Name is what a player reads, and Shade is the darker tone the drawing uses
// for the same surface's shadow.
//
// Hex and Shade are empty for the "same as the fur" entries, which resolve to
// whatever the fur is.
type Colour struct {
	Key   string
	Name  string
	Hex   string
	Shade string
}

// Option is a face or a piece of bling: a key and what a player reads.
type Option struct {
	Key  string
	Name string
}

// SameAsFur is the key of the palette entry that takes the fur's colour. It is
// the default for the headdress and the handle, because that is how the
// mascot is drawn.
const SameAsFur = "fur"

// The palettes below were picked by measurement, with the same dE76 the blade
// colours in app.css were picked by (docs/adr/0029):
//
//   - every fur at least 38 from every blade, so the blade does not sink into
//     the cat, and at least 56 from the page background (--bg, #0a0a12);
//   - every headdress at least 74 from the outline (#080808), because its
//     stripes alternate with black ones;
//   - every handle at least 58 from the outline, because it is small and
//     framed by it.
//
// Each shade takes the step the drawing takes from #f89828 to #b85818,
// measured in CIELAB, except ginger's: that step tips it into red, so its
// shade is set by hand.
//
// Order matters only for display, except in Blades, see there.

// Furs is the coat. The first entry is the drawing's own.
var Furs = []Colour{
	{"orange", "Orange", "#f89828", "#b85818"},
	{"ginger", "Ingwer", "#d9622b", "#98401a"},
	{"cinnamon", "Zimt", "#b8743a", "#7b391a"},
	{"gold", "Gold", "#f2c230", "#ac8622"},
	{"sand", "Sand", "#d8b98a", "#977d5d"},
	{"cream", "Creme", "#f2e3c6", "#b0a590"},
	{"white", "Weiß", "#f7f5f0", "#b7b5b2"},
	{"chocolate", "Schoko", "#94603a", "#5a2816"},
}

// Headdresses is the coloured stripes of the headdress.
var Headdresses = []Colour{
	{SameAsFur, "Wie das Fell", "", ""},
	{"gold", "Gold", "#fdc82d", "#b68b22"},
	{"lapis", "Lapis", "#3783e6", "#27498f"},
	{"turquoise", "Türkis", "#17b3b7", "#007976"},
	{"emerald", "Smaragd", "#35b567", "#007b3d"},
	{"ruby", "Rubin", "#d9343f", "#a1001e"},
	{"ivory", "Elfenbein", "#f2ead8", "#b1ab9e"},
	{"silver", "Silber", "#b9bcc4", "#7e8085"},
}

// Handles is the grip of the bat.
var Handles = []Colour{
	{SameAsFur, "Wie das Fell", "", ""},
	{"wood-light", "Holz hell", "#d9a86c", "#996c46"},
	{"wood-dark", "Holz dunkel", "#9a6438", "#5f2c15"},
	{"red", "Rot", "#c82828", "#8f000b"},
	{"blue", "Blau", "#2f6fd0", "#21367b"},
	{"green", "Grün", "#35b567", "#007b3d"},
	{"ivory", "Elfenbein", "#f2ead8", "#b1ab9e"},
}

// Blades is the face of the bat: the seven colours of .paddle-0 to .paddle-6
// in app.css, in that order. The order is load-bearing. A player who has not
// chosen gets the blade at the index templates.PaddleClass derives from the
// id, and zaehlwerk looks the same index up in its own copy of this list.
//
// The drawing's red is not among them. It sits dE 26 from the colour that
// means a loss, and the blade keeps 37 from every colour that means something
// here. It stays the blade of a browser nobody is recognised in.
var Blades = []Colour{
	{"light-blue", "Hellblau", "#3aa6ff", ""},
	{"blue", "Blau", "#2f6fd0", ""},
	{"petrol", "Petrol", "#159c8c", ""},
	{"moss", "Moosgrün", "#4d7c4a", ""},
	{"pink", "Pink", "#d64f9c", ""},
	{"orchid", "Orchidee", "#c05fd0", ""},
	{"steel", "Stahl", "#8c8f9c", ""},
}

// Faces is the expressions, neutral first: it is the face the mascot has
// always had, and the one a figure wears away from a running match. The others
// are for the game, and which one shows when is the Zählwerk's call, not a
// player's (docs/adr/0029).
var Faces = []Option{
	{"neutral", "Neutral"},
	{"happy", "Fröhlich"},
	{"surprised", "Überrascht"},
	{"skeptical", "Skeptisch"},
	{"angry", "Wütend"},
	{"sad", "Traurig"},
}

// NoBling is the key for wearing nothing.
const NoBling = ""

// Blings is what the figure may wear. Free for everybody: none of it is an
// award.
var Blings = []Option{
	{NoBling, "Kein Schmuck"},
	{"collar", "Pharaonenkragen"},
	{"scarab", "Skarabäus"},
	{"ankh", "Anch"},
	{"horus-eye", "Horusauge"},
	{"chain", "Champion-Kette"},
	{"ruby", "Rubin-Talisman"},
}

func findColour(p []Colour, key string) (Colour, bool) {
	for _, c := range p {
		if c.Key == key {
			return c, true
		}
	}
	return Colour{}, false
}

func hasOption(p []Option, key string) bool {
	for _, o := range p {
		if o.Key == key {
			return true
		}
	}
	return false
}
