package server_test

import (
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/stuttgart-things/schmetterpause/internal/server"
)

// A malformed bracket is "no tournament", not a rejection. The field rides
// along on every entry path, and refusing the whole result over it would lose
// a match somebody just played — which at the kiosk means asking two people
// to play it again.
func TestTournamentIDFromToleratesRubbish(t *testing.T) {
	id := uuid.New()

	for _, tc := range []struct {
		name  string
		value string
		want  *uuid.UUID
	}{
		{"absent", "", nil},
		{"blank", "   ", nil},
		{"not a uuid", "schnelles-turnier", nil},
		{"truncated", id.String()[:20], nil},
		{"a real one", id.String(), &id},
		{"padded", "  " + id.String() + "  ", &id},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := url.Values{"tournament_id": {tc.value}}.Encode()
			r := httptest.NewRequest("POST", "/", strings.NewReader(body))
			r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

			got := server.TournamentIDFrom(r)
			switch {
			case tc.want == nil && got != nil:
				t.Errorf("got %v, want nil", got)
			case tc.want != nil && (got == nil || *got != *tc.want):
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestCheckFieldRefusesWhatCannotBePlayed(t *testing.T) {
	ids := func(n int) []uuid.UUID {
		out := make([]uuid.UUID, n)
		for i := range out {
			out[i] = uuid.New()
		}
		return out
	}

	if msg := server.CheckField(nil); msg == "" {
		t.Error("an empty field was accepted")
	}
	if msg := server.CheckField(ids(1)); msg == "" {
		t.Error("a field of one was accepted — there is nothing to play")
	}
	if msg := server.CheckField(ids(2)); msg != "" {
		t.Errorf("two players were refused: %q", msg)
	}
	if msg := server.CheckField(ids(server.MaxTournamentPlayers)); msg != "" {
		t.Errorf("the cap itself was refused: %q", msg)
	}
	// One past the cap is a tournament nobody can finish, and the number is
	// easier to argue with before the draw than after.
	if msg := server.CheckField(ids(server.MaxTournamentPlayers + 1)); msg == "" {
		t.Error("a field past the cap was accepted")
	}
}
