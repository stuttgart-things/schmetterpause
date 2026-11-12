package avatar

// FurPalette is the selectable main surface for a character. The original
// material of new characters is the separate empty key. blecki keeps cool or
// muted wolf tones; the remaining alternatives are subject to visual review.
func FurPalette(characterKey string) []Colour {
	if characterKey == "pip" {
		return PipFurs
	}
	if characterKey != "vex" {
		return Furs
	}
	allowed := map[string]bool{"sand": true, "white": true, "chocolate": true}
	palette := make([]Colour, 0, len(allowed))
	for _, c := range Furs {
		if allowed[c.Key] {
			palette = append(palette, c)
		}
	}
	return palette
}

func validFur(characterKey, key string) bool {
	return has(FurPalette(characterKey), key) || (characterKey != "" && key == "")
}

// BleckiBlings uses the existing single stored slot, with its own drawing kit.
var BleckiBlings = []Option{
	{NoBling, "Kein Schmuck"},
	{"ball", "Tischtennisball"},
}

// BlingPalette returns the jewellery drawn for this character’s single slot.
var HankBlings = []Option{{NoBling, "Kein Schmuck"}, {"initial", "Initiale"}}

func BlingPalette(characterKey string) []Option {
	if characterKey == "pip" {
		return HankBlings
	}
	if characterKey == "vex" {
		return BleckiBlings
	}
	if characterKey == "" {
		return Blings
	}
	return []Option{{NoBling, "Kein Schmuck"}}
}

// PipFurs keep the face/bib distinct and the coat visible on a black panel.
var PipFurs = []Colour{
	{"grey", "Grau", "#8793a5", "#5a6678"},
	{"blue-grey", "Blaugrau", "#587e9b", "#38556e"},
}

// HeaddressPalette retains shared colours except Pip's navy-adjacent Lapis.
// With a coat supplied, grey also excludes Silver. Pip's native pink is Original.
func HeaddressPalette(characterKey string, fur ...string) []Colour {
	if characterKey != "pip" {
		return Headdresses
	}
	var palette []Colour
	for _, c := range Headdresses {
		if c.Key == SameAsFur {
			c.Name = "Original"
		}
		if c.Key != "lapis" && !(len(fur) > 0 && fur[0] == "grey" && c.Key == "silver") {
			palette = append(palette, c)
		}
	}
	return palette
}
