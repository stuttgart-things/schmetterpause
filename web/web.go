// Package web embeds the static assets into the binary.
//
// Invariant 1 in CLAUDE.md calls for one binary and one image across all
// target environments. Loading assets from the filesystem would tie the image
// to a volume, so CSS, HTMX and the fonts live inside the binary.
//
// The fonts are here rather than on a font CDN for the same reason the
// configuration has no hardcoded hosts (invariant 2): the application must
// come up on a network that cannot reach the internet, and a typeface that
// silently falls back is a rendering bug nobody reports. Two subsets each of
// Space Grotesk and JetBrains Mono, 84 kB in total, licences alongside them.
//
// The mark and the mascot under static/img are the same reasoning again,
// and 130 kB of it: an icon fetched from somewhere else is an icon that is
// missing on a closed network.
package web

import "embed"

// Static holds web/static.
//
//go:embed static
var Static embed.FS

// Mascot is the mark itself, for inlining into a page.
//
// Inlined rather than referenced with <img src>: an SVG loaded through <img>
// is an isolated document that the surrounding CSS cannot reach, and the
// point of the vector version is that the blade can be recoloured from
// outside through --paddle. See issue #64.
//
// It is also still served as a static file, so anything that only wants to
// display it — a README, a chat message — can link to it. That means the 7.6 kB
// are in the binary twice, once in Static and once here. Deliberate: reading it
// out of Static at init instead would save the copy but turn a missing file
// from a compile error into a crash on startup, and the file is small enough
// that the guarantee is worth more.
//
//go:embed static/img/mascot.svg
var Mascot string
