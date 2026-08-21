// Package domain enthaelt die fachlichen Typen der Anwendung.
//
// Das Package haengt bewusst weder von der Datenbank noch von HTTP ab, damit
// Fachlogik (TTR-Berechnung, Tabellen, Brackets) ohne Infrastruktur testbar
// bleibt — siehe Abschnitt "Konventionen" in CLAUDE.md.
package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// ErrNotFound meldet, dass ein angefragter Datensatz nicht existiert.
// Repository-Implementierungen wrappen ihn, Aufrufer pruefen mit errors.Is.
var ErrNotFound = errors.New("nicht gefunden")

// DefaultTTR ist der Startwert fuer neu angelegte Spieler.
//
// Ob 1000 fuer eine Buerogruppe gut streut, ist eine offene Frage des
// MVP-Plans und wird erst mit echten Daten beantwortet.
const DefaultTTR = 1000

// Provider benennt das Verfahren, ueber das eine Identitaet nachgewiesen wurde.
// Neue Verfahren sind ein Datensatz, kein Schema-Change (docs/adr/0003).
type Provider string

const (
	// ProviderLocal ist die Wiedererkennung ueber ein signiertes Cookie ohne
	// echte Authentifizierung. Im MVP der einzige Provider.
	ProviderLocal Provider = "local"
	// ProviderGitLab ist OIDC gegen das Firmen-GitLab. Noch nicht umgesetzt.
	ProviderGitLab Provider = "gitlab"
	// ProviderGitHub ist OIDC gegen GitHub. Noch nicht umgesetzt.
	ProviderGitHub Provider = "github"
	// ProviderPasskey ist WebAuthn. Speichert nur Public Key und
	// Signaturzaehler, niemals biometrische Merkmale (docs/adr/0004).
	ProviderPasskey Provider = "passkey"
)

// Player ist ein Spieler. Die Struktur enthaelt ausschliesslich fachliche
// Daten; wie ein Spieler authentifiziert wurde, steht in Identity.
type Player struct {
	ID          uuid.UUID
	DisplayName string
	TTR         int
	CreatedAt   time.Time
}

// Identity verknuepft einen Nachweis eines Providers mit einem Spieler.
// Ein Spieler kann mehrere Identitaeten haben.
type Identity struct {
	Provider  Provider
	Subject   string
	PlayerID  uuid.UUID
	CreatedAt time.Time
}

// MatchStatus ist der Bestaetigungsstand eines Matches. Nur ein Match im
// Status MatchConfirmed geht in die TTR-Wertung ein.
type MatchStatus string

const (
	// MatchPending ist eingetragen, aber vom Gegner noch nicht bestaetigt.
	MatchPending MatchStatus = "pending"
	// MatchConfirmed ist vom Gegner bestaetigt und wird gewertet.
	MatchConfirmed MatchStatus = "confirmed"
	// MatchDisputed ist vom Gegner bestritten und blockiert die Wertung.
	// Im MVP nur manuell aufloesbar.
	MatchDisputed MatchStatus = "disputed"
)

// Match ist eine Einzelbegegnung zwischen zwei Spielern. Doppel zaehlen nicht
// fuer TTR und brauchen, falls sie kommen, eine eigene Wertung.
type Match struct {
	ID          uuid.UUID
	HomeID      uuid.UUID
	AwayID      uuid.UUID
	BestOf      int
	PointsToWin int
	Status      MatchStatus
	ReportedBy  uuid.UUID
	PlayedAt    time.Time
	// ConfirmedAt ist genau dann gesetzt, wenn Status MatchConfirmed ist.
	ConfirmedAt *time.Time
	Sets        []MatchSet
}

// MatchSet ist ein einzelner Satz innerhalb eines Matches.
type MatchSet struct {
	SetNo      int
	HomePoints int
	AwayPoints int
}

// TTRChange ist ein Eintrag der TTR-Historie: die Veraenderung eines
// Spielerratings durch genau ein gewertetes Match.
type TTRChange struct {
	ID        uuid.UUID
	PlayerID  uuid.UUID
	MatchID   uuid.UUID
	TTRBefore int
	TTRAfter  int
	CreatedAt time.Time
}

// Delta ist die Ratingaenderung dieses Eintrags.
func (c TTRChange) Delta() int { return c.TTRAfter - c.TTRBefore }
