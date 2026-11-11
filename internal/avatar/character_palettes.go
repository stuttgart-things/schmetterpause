package avatar

// FurPalette is the selectable main surface for a character. The original
// material of new characters is the separate empty key. Blecki keeps cool or
// muted wolf tones; the remaining alternatives are subject to visual review.
func FurPalette(characterKey string) []Colour {
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
	{"bandana", "Halstuch"},
	{"captain", "Kapitänsbinde"},
	{"moon", "Mond-Anhänger"},
	{"wolf-tooth", "Wolfszahn-Kette"},
}

// BlingPalette returns the jewellery drawn for this character’s single slot.
func BlingPalette(characterKey string) []Option {
	if characterKey == "vex" {
		return BleckiBlings
	}
	if characterKey == "" {
		return Blings
	}
	return []Option{{NoBling, "Kein Schmuck"}}
}
