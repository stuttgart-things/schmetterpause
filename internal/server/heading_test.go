package server_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stuttgart-things/schmetterpause/internal/seed"
)

// TestEveryPageOpensWithExactlyOneH1 is the guard against the bug it is named
// after.
//
// /statistics and /tournaments were built without an h1 at all: their first
// heading was an h2 naming the page, so the outline began one level down and
// had no top to jump to — which is how a page is skimmed without sight. It
// also held the stylesheet hostage, because a card heading that doubles as a
// page title cannot be sized like a card heading (issue #245).
//
// Nothing in the build could see it. Both pages rendered, both looked
// plausible, and the words were even already there — Layout has been given
// each page's name all along.
//
// Rendered rather than read off the sources, because what matters is what
// arrives at the browser: a page head that a handler forgets to pass, or a
// component that stops being included, is the same bug and this catches it.
func TestEveryPageOpensWithExactlyOneH1(t *testing.T) {
	ctx := context.Background()
	store := newMemStore()
	// The fixture from issue #82, which is also what the screenshots use: it
	// fills the pages that are empty shells otherwise, and it brings a running
	// tournament along, so the one page with an id in its path is covered too.
	if _, err := seed.Run(ctx, store, time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	running, err := store.Tournaments().List(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(running) == 0 {
		t.Fatal("the fixture brought no tournament; the tournament page would go unchecked")
	}
	h := newHandler(store)

	for _, path := range []string{
		"/",
		"/standings",
		"/statistics",
		"/tournaments",
		"/tournaments/" + running[0].ID.String(),
		"/matches",
		"/info",
		"/rules",
		"/qr",
	} {
		body := get(t, h, path).Body.String()

		if n := strings.Count(body, "<h1>"); n != 1 {
			t.Errorf("%s carries %d h1, want exactly 1 — a page has one name", path, n)
			continue
		}
		// And it is the top of the outline, not something further down it.
		// An h2 above the h1 reads as the page title to anything that
		// navigates by heading, which is the half of the bug a count alone
		// would miss.
		if h2 := strings.Index(body, "<h2>"); h2 >= 0 && h2 < strings.Index(body, "<h1>") {
			t.Errorf("%s opens on an h2 rather than its h1", path)
		}
	}
}
