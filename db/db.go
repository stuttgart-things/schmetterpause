// Package db embeds the database migrations into the binary.
//
// The migrations belong in the image, not in a sidecar: invariant 1 in
// CLAUDE.md calls for one binary and one image across all target
// environments, and the pipeline's verify step checks exactly that — a built
// image that brings its own migrations along.
package db

import "embed"

// Migrations holds the goose migrations from db/migrations.
//
//go:embed migrations/*.sql
var Migrations embed.FS
