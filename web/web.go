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
