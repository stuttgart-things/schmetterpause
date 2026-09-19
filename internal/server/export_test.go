package server

import (
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/stuttgart-things/schmetterpause/internal/domain"
)

// WaitInWords exposes the wording helper to the package's external tests.
// The German it produces is read by somebody who has just been refused, and
// a test that has to go through an HTTP round trip to check a plural is a
// test nobody writes.
func WaitInWords(d time.Duration) string { return waitInWords(d) }

// TournamentIDFrom and CheckField are exposed for the same reason: both
// decide what happens to a result somebody just typed at the table, and both
// are pure enough that an HTTP round trip would only obscure the case.
func TournamentIDFrom(r *http.Request) *uuid.UUID { return tournamentIDFrom(r) }

func CheckField(field []uuid.UUID) string { return checkField(field) }

// WaitedSince is exposed for the same reason as WaitInWords: it decides what
// a reader is told about how long a result has been sitting, the buckets have
// edges worth pinning down, and none of that is easier to see through a page
// of HTML.
func WaitedSince(now, playedAt time.Time) (string, bool) { return waitedSince(now, playedAt) }

// StaleAfter is when a waiting result stops counting as normal.
const StaleAfter = staleAfter

// MaxTournamentPlayers is the cap the form and the check share.
const MaxTournamentPlayers = maxTournamentPlayers

// WaitingOn is exposed because its third case — a reporter who is neither
// side — only arrives through the Zählwerk's token-gated API, and building
// that round trip to check one word would bury the case it is about.
func WaitingOn(m domain.Match, names map[uuid.UUID]string) string { return waitingOn(m, names) }
