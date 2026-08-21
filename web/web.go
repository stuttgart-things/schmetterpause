// Package web embeds the static assets into the binary.
//
// Invariant 1 in CLAUDE.md calls for one binary and one image across all
// target environments. Loading assets from the filesystem would tie the image
// to a volume, so CSS and HTMX live inside the binary.
package web

import "embed"

// Static holds web/static.
//
//go:embed static
var Static embed.FS
