---
name: screenshots
description: Look at the running application without a database — serve it from the in-memory store and the seed fixture, then drive a browser at it. Use when a change touches templates or app.css, when a claim about layout needs checking, or when a pull request should carry before/after pictures.
---

A change to `app.css` or a `.templ` file is not finished when the tests pass. The tests say the markup is there; they say nothing about whether a control fits on a phone. This is how to see it, and it takes about a minute.

**There is no Docker in an agent sandbox**, so `task up:local` and Compose are not available. They are also not needed: invariant 5 puts data access behind repository interfaces, and the test suite already has a full in-memory implementation.

## The harness

Write `internal/server/zz_shot_test.go`. It is scratch — **delete it before committing**, and keep the `zz_` prefix so it sorts last and nobody mistakes it for a test.

```go
package server_test

// It has to live in package server_test: newMemStore and newHandlerWith are
// unexported test helpers, and this is the only place they are visible.

type asPlayer struct{ id uuid.UUID }

// Identify returns the PLAYER id, not the session subject. The cookie
// authenticator resolves subject to player internally, so a fake that hands
// back a subject renders every page signed out and looks like a seeding bug.
func (a asPlayer) Identify(*http.Request) (uuid.UUID, error)         { return a.id, nil }
func (asPlayer) SetCookie(http.ResponseWriter, string)               {}
func (asPlayer) ClearCookie(http.ResponseWriter)                     {}
func (asPlayer) EndSession(http.ResponseWriter, *http.Request) error { return nil }

func TestServeForScreenshots(t *testing.T) {
	if os.Getenv("SHOT_SERVE") == "" {
		t.Skip("set SHOT_SERVE")
	}
	ctx := context.Background()
	store := newMemStore()
	// The fixture from issue #82, built for exactly this. Deterministic, so
	// two shots of the same commit are identical and a diff is the change
	// rather than noise. It brings six players, a dozen settled matches and a
	// running tournament.
	seed.Run(ctx, store, time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC))
	players, _ := store.Players().List(ctx)

	// Two ports, because half the interesting pages only exist for somebody
	// the browser is recognised as, and the other half only for a stranger.
	anon, _ := net.Listen("tcp", "127.0.0.1:8099")
	known, _ := net.Listen("tcp", "127.0.0.1:8098")
	go (&http.Server{Handler: newHandlerWith(store, auth.Anonymous{})}).Serve(anon)
	go (&http.Server{Handler: newHandlerWith(store, asPlayer{id: players[3].ID})}).Serve(known)

	time.Sleep(600 * time.Second)
}
```

Start it in the background with `SHOT_SERVE=1 go test ./internal/server/ -run TestServeForScreenshots -count=1`, and wait for the port rather than sleeping blindly: `until curl -sf -o /dev/null http://127.0.0.1:8099/; do sleep 2; done`.

`web.go` embeds the stylesheet, so **an edit to `app.css` needs the harness restarted** before it shows up.

## Driving the browser

Use Playwright, not the Chromium CLI. `chrome --headless --screenshot --window-size=390,800` measures the window in *device* pixels, so with `--force-device-scale-factor=2` the page lays out at 195px and everything looks broken for a reason that is not in the diff.

```js
const ctx = await browser.newContext({ viewport: { width: 390, height: 844 }, deviceScaleFactor: 2 });
await page.screenshot({ path: out, fullPage: true });
```

390×844 is the phone this application is actually used on; 1000 wide is the second width worth taking. Chromium ships in the sandbox (`/opt/pw-browsers`, `PLAYWRIGHT_BROWSERS_PATH`) — pass it as `executablePath` and never run `playwright install`.

## Two measurements worth more than looking

**Horizontal overflow**, which is the failure mode this stylesheet cares about most (`.table-scroll` exists for it):

```js
await page.evaluate(() => ({ scroll: document.documentElement.scrollWidth,
                             client: document.documentElement.clientWidth }))
```

Equal on every page at 390px is the claim to make in a pull request. Unequal is a bug, and the same `evaluate` can walk `document.querySelectorAll('body *')` for the element whose `getBoundingClientRect().right` exceeds the viewport.

**A page that should not have changed.** Shoot before and after into two directories and `cmp` the PNGs. Byte-identical is a far stronger statement than "looks the same to me", and it is how a change to a shared selector proves its blast radius. `/info` and `/standings` came back byte-identical across a stylesheet change that touched five other pages, which is what made that change easy to review.

Computed values settle arguments the same way: `getComputedStyle(el).fontSize` is how the heading scale in #245 was shown to be 1.6px rather than a matter of taste.

## In a pull request

Screenshots cannot be attached to a comment from here, so publish them as an artifact and link it from the body. Put before and after side by side, label which branch each side is, and say what to look at — a reviewer should not have to find the difference.
