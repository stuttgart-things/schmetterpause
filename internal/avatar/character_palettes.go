package avatar

// FurPalette is the selectable main surface for a character. The original
// material of new characters is the separate empty key. Vex keeps cool or
// muted wolf tones; the remaining alternatives are subject to visual review.
func FurPalette(characterKey string) []Colour {
	if characterKey != "vex" {
		return Furs
	}
	allowed := map[string]bool{"cinnamon": true, "sand": true, "white": true, "chocolate": true}
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
