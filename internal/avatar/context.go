package avatar

import (
	"strings"
	"unicode"
)

// RenderContext is transient identity information, never an avatar choice or
// database column. Without it the hero emblem uses its racket fallback.
type RenderContext struct{ DisplayName string }

// InitialForName accepts only the supported first character, not a later letter.
func InitialForName(name string) string {
	for _, r := range strings.TrimSpace(name) {
		r = unicode.ToUpper(r)
		if r >= 'A' && r <= 'Z' || r == 'Ä' || r == 'Ö' || r == 'Ü' {
			return string(r)
		}
		break
	}
	return ""
}

func HankEmblemColor(a Avatar) string {
	a = Fill(a, "")
	ch, _ := character("pip")
	fur, _ := findColour(FurPalette("pip"), a.Fur)
	if a.Headdress == "ivory" || a.Headdress == "silver" {
		return orFur(fur.Hex, ch.Fur)
	}
	head, _ := findColour(HeaddressPalette("pip", a.Fur), a.Headdress)
	return orFur(head.Hex, ch.Headdress)
}
