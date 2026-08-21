// Package web bettet die statischen Assets in das Binary ein.
//
// Invariante 1 aus CLAUDE.md verlangt ein Binary und ein Image fuer alle
// Zielumgebungen. Assets aus dem Dateisystem nachzuladen wuerde das Image an
// ein Volume binden; deshalb liegen CSS und HTMX im Binary.
package web

import "embed"

// Static enthaelt web/static.
//
//go:embed static
var Static embed.FS
