package server_test

import (
	"strings"
	"testing"

	"github.com/stuttgart-things/schmetterpause/internal/auth"
	"github.com/stuttgart-things/schmetterpause/internal/domain"
)

// The first week's feedback, in as many words: with no account yet, getting
// one was the confusing part. The start page opens on a picker of names a
// stranger is not in, and the way to become one of them sat at the bottom of
// the card — found, the second round of feedback said, by nobody new.
func TestAStrangerIsToldWhichCardIsTheirs(t *testing.T) {
	store := newMemStore()
	h := newHandlerWith(store, auth.NewCookieAuthenticator(store.Identities(), testSessionKey, false))

	if _, err := store.Players().Create(t.Context(), "Anna", domain.DefaultTTR); err != nil {
		t.Fatalf("Create(): %v", err)
	}

	body := get(t, h, "/").Body.String()

	if !strings.Contains(body, "Neu hier?") {
		t.Errorf("nothing on the sign-in card speaks to somebody new: %s", body)
	}
	// Before the list of names, not after it (docs/adr/0018).
	newDoor := strings.Index(body, `class="linklike" hx-get="/fragments/join"`)
	picker := strings.Index(body, `name="player_id"`)
	if newDoor < 0 {
		t.Fatalf("joining is not offered on the sign-in card: %s", body)
	}
	if picker >= 0 && newDoor > picker {
		t.Error("the way in for somebody new comes after the list of names")
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
	if !strings.Contains(body, "keine E-Mail") {
		t.Errorf("the join card does not say how little it asks: %s", body)
	}
	for _, field := range []string{`name="display_name"`, `name="pin"`} {
		if !strings.Contains(body, field) {
			t.Errorf("the join card lacks %s: %s", field, body)
		}
	}
	// A PIN field with nothing beside it is a field somebody stares at.
	if !strings.Contains(body, `class="pin-example"`) {
		t.Errorf("the PIN field comes without an example: %s", body)
	}
}
