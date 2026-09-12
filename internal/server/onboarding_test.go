package server_test

import (
	"strings"
	"testing"

	"github.com/stuttgart-things/schmetterpause/internal/auth"
	"github.com/stuttgart-things/schmetterpause/internal/domain"
)

// The first week's feedback, in as many words: with no account yet, getting
// one was the confusing part. The start page opens on a picker of names a
// stranger is not in, and the way to become one of them was a single
// underlined word at the bottom of the card.
func TestAStrangerIsToldWhichCardIsTheirs(t *testing.T) {
	store := newMemStore()
	h := newHandlerWith(store, auth.NewCookieAuthenticator(store.Identities(), testSessionKey, false))

	if _, err := store.Players().Create(t.Context(), "Anna", domain.DefaultTTR); err != nil {
		t.Fatalf("Create(): %v", err)
	}

	body := get(t, h, "/").Body.String()

	// The card says who it is for, rather than leaving it to be worked out
	// from a list of names.
	if !strings.Contains(body, "Du stehst schon in der Rangliste") {
		t.Errorf("the sign-in card does not say who it is for: %s", body)
	}
	// And the other door carries a button's weight now. Still the outlined
	// kind, so the two are not presented as equal choices.
	if !strings.Contains(body, `class="secondary" hx-get="/fragments/join"`) {
		t.Errorf("joining is not offered as a visible control: %s", body)
	}
	if !strings.Contains(body, "Noch gar kein Spieler?") {
		t.Errorf("nothing on the page says what the second door is: %s", body)
	}
}

// And once somebody is behind that door, the form says what it costs. A lone
// field with a button says nothing about whether a password comes next.
func TestTheJoinFormSaysWhatItAsksFor(t *testing.T) {
	store := newMemStore()
	h := newHandlerWith(store, auth.NewCookieAuthenticator(store.Identities(), testSessionKey, false))

	body := get(t, h, "/fragments/join").Body.String()

	if !strings.Contains(body, "Zum ersten Mal hier?") {
		t.Errorf("the join card does not say what it is: %s", body)
	}
	if !strings.Contains(body, "Kein Passwort") {
		t.Errorf("the join card does not say that a name is the whole of it: %s", body)
	}
	if !strings.Contains(body, `name="display_name"`) {
		t.Errorf("the join card lost its field: %s", body)
	}
}
