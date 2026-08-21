// Package db bettet die Datenbank-Migrations in das Binary ein.
//
// Die Migrations gehoeren ins Image, nicht in ein Sidecar: Invariante 1 aus
// CLAUDE.md verlangt ein Binary und ein Image fuer alle Zielumgebungen, und der
// Verify-Schritt der Pipeline prueft genau, dass das gebaute Image seine
// Migrations selbst mitbringt.
package db

import "embed"

// Migrations enthaelt die goose-Migrations aus db/migrations.
//
//go:embed migrations/*.sql
var Migrations embed.FS
