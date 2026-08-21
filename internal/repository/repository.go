// Package repository definiert den Datenzugriff als Schnittstelle.
//
// Invariante 5 aus CLAUDE.md: kein SQL in Handlern. Das haelt den in
// docs/adr/0001 beschriebenen Wechselpfad (Postgres -> SQLite, falls das
// Portabilitaetsziel je entfaellt) offen und macht Handler ohne Datenbank
// testbar.
package repository

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/stuttgart-things/schmetterpause/internal/domain"
)

// Store buendelt die Repositories und die Transaktionsklammer.
type Store interface {
	Players() PlayerRepository
	Identities() IdentityRepository
	Matches() MatchRepository
	TTRHistory() TTRHistoryRepository

	// InTx fuehrt fn in einer Transaktion aus. Der uebergebene Store schreibt
	// auf dieselbe Transaktion; gibt fn einen Fehler zurueck, wird
	// zurueckgerollt. Notwendig fuer AP5: TTR schreiben, Historie anlegen und
	// Match bestaetigen muessen zusammen gelingen oder zusammen scheitern.
	InTx(ctx context.Context, fn func(Store) error) error

	// Ping prueft die Erreichbarkeit des Backends. Ziel ist /readyz.
	Ping(ctx context.Context) error
}

// PlayerRepository verwaltet Spieler.
type PlayerRepository interface {
	Create(ctx context.Context, displayName string, initialTTR int) (domain.Player, error)
	ByID(ctx context.Context, id uuid.UUID) (domain.Player, error)
	// List liefert alle Spieler, absteigend nach TTR — die Reihenfolge der
	// Rangliste aus AP6.
	List(ctx context.Context) ([]domain.Player, error)
	Count(ctx context.Context) (int, error)
	UpdateTTR(ctx context.Context, id uuid.UUID, ttr int) error
}

// IdentityRepository verknuepft Provider-Nachweise mit Spielern.
// Ausserhalb dieses Interfaces und des auth-Packages kennt niemand die
// konkreten Provider (Invariante 4).
type IdentityRepository interface {
	// Link legt die Verknuepfung an. Existiert sie bereits fuer denselben
	// Spieler, ist der Aufruf ein No-op.
	Link(ctx context.Context, provider domain.Provider, subject string, playerID uuid.UUID) error
	// PlayerBy liefert den Spieler hinter einem Nachweis. Ist keiner
	// verknuepft, ist der Fehler domain.ErrNotFound.
	PlayerBy(ctx context.Context, provider domain.Provider, subject string) (domain.Player, error)
	ForPlayer(ctx context.Context, playerID uuid.UUID) ([]domain.Identity, error)
}

// MatchRepository verwaltet Begegnungen samt Saetzen.
type MatchRepository interface {
	// Create legt Match und Saetze gemeinsam an und liefert den
	// persistierten Stand inklusive vergebener ID zurueck.
	Create(ctx context.Context, m domain.Match) (domain.Match, error)
	ByID(ctx context.Context, id uuid.UUID) (domain.Match, error)
	// PendingFor liefert die Matches, die auf die Bestaetigung durch diesen
	// Spieler warten — also solche, die jemand anderes eingetragen hat.
	PendingFor(ctx context.Context, playerID uuid.UUID) ([]domain.Match, error)
	// RecentFor liefert die juengsten Matches eines Spielers.
	RecentFor(ctx context.Context, playerID uuid.UUID, limit int) ([]domain.Match, error)
	// SetStatus schreibt den Bestaetigungsstand. confirmedAt ist genau dann
	// gesetzt, wenn status domain.MatchConfirmed ist.
	SetStatus(ctx context.Context, id uuid.UUID, status domain.MatchStatus, confirmedAt *time.Time) error
}

// TTRHistoryRepository schreibt und liest die Ratinghistorie.
type TTRHistoryRepository interface {
	// Append schreibt die Eintraege einer Wertung. Aufrufer nutzen InTx,
	// damit Historie und Spielerrating nicht auseinanderlaufen koennen.
	Append(ctx context.Context, changes []domain.TTRChange) error
	ForPlayer(ctx context.Context, playerID uuid.UUID, limit int) ([]domain.TTRChange, error)
}
