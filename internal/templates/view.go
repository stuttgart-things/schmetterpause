// Package templates enthaelt die templ-Templates und ihre View-Modelle.
//
// Handler geben HTML-Fragmente zurueck, keine JSON-APIs (CLAUDE.md,
// "Konventionen"). Die generierten *_templ.go-Dateien sind eingecheckt; der
// Lint-Schritt der Pipeline prueft, dass sie zum Stand der *.templ-Dateien
// passen.
package templates

// generate erzeugt die *_templ.go-Dateien neu. templ ist als Go-Tool in
// go.mod gepinnt, damit lokal und in der Pipeline dieselbe Version laeuft.
//
//go:generate go tool templ generate

// StatusView ist das Modell des Statusfragments auf der Startseite. Es dient
// im Geruest als Nachweis, dass Template, HTMX und Repository zusammenspielen.
type StatusView struct {
	// Players ist die Anzahl angelegter Spieler.
	Players int
	// DatabaseReachable meldet, ob die Datenbank auf einen Ping antwortet.
	DatabaseReachable bool
	// Version ist die Version des laufenden Binaries.
	Version string
}
