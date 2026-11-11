package avatar

import "fmt"

// BallColors is blecki's ball palette. Empty means white, including legacy rows.
// Each shadow is calculated at seven eighths of the selected RGB channels.
var BallColors = []Colour{
	ballTone("", "Weiß", 248, 248, 248),
	ballTone("orange", "Orange", 255, 138, 36),
	ballTone("yellow", "Gelb", 255, 226, 59),
}

func ballTone(key, name string, r, g, b uint8) Colour {
	shade := func(v uint8) uint8 { return uint8(uint16(v) * 7 / 8) }
	return Colour{Key: key, Name: name, Hex: fmt.Sprintf("#%02x%02x%02x", r, g, b), Shade: fmt.Sprintf("#%02x%02x%02x", shade(r), shade(g), shade(b))}
}

// NormalizeBallColor tolerates absent and unknown keys by selecting white.
func NormalizeBallColor(key string) string {
	if _, ok := findColour(BallColors, key); ok {
		return key
	}
	return ""
}

// BallTone resolves the selected surface and its derived same-hue shadow.
func BallTone(key string) Colour {
	c, _ := findColour(BallColors, NormalizeBallColor(key))
	return c
}
