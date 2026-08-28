package ratelimit_test

import (
	"sync"
	"testing"
	"time"

	"github.com/stuttgart-things/schmetterpause/internal/ratelimit"
)

// clock is a hand-wound wall clock, so a test can let an hour pass without
// taking one.
type clock struct {
	mu sync.Mutex
	at time.Time
}

func newClock() *clock {
	return &clock{at: time.Date(2026, 8, 28, 9, 0, 0, 0, time.UTC)}
}

func (c *clock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.at
}

func (c *clock) pass(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.at = c.at.Add(d)
}

func testPolicy() ratelimit.Policy {
	return ratelimit.Policy{
		Free:   2,
		Step:   time.Second,
		Max:    16 * time.Second,
		Forget: time.Hour,
	}
}

func TestAFreshKeyGoesAhead(t *testing.T) {
	t.Parallel()

	l := ratelimit.New(testPolicy())
	if got := l.Retry("anna"); got != 0 {
		t.Errorf("Retry() on a key that never failed = %s, want 0", got)
	}
}

// The free attempts are not generosity, they are the point. Somebody mistyping
// a sixteen-character code is the person this whole way back exists for, and
// a stopwatch on their second try would punish them for it.
func TestTheFirstFailuresCostNothing(t *testing.T) {
	t.Parallel()

	c := newClock()
	l := ratelimit.NewAt(testPolicy(), c.now)

	for i := range 2 {
		l.Failed("anna")
		if got := l.Retry("anna"); got != 0 {
			t.Errorf("after failure %d, Retry() = %s, want 0", i+1, got)
		}
	}
}

func TestTheWaitDoubles(t *testing.T) {
	t.Parallel()

	c := newClock()
	l := ratelimit.NewAt(testPolicy(), c.now)

	// Free is 2, so the third failure is the first to cost anything.
	want := []time.Duration{time.Second, 2 * time.Second, 4 * time.Second, 8 * time.Second}

	l.Failed("anna")
	l.Failed("anna")

	for _, w := range want {
		l.Failed("anna")
		if got := l.Retry("anna"); got != w {
			t.Fatalf("Retry() = %s, want %s", got, w)
		}
		c.pass(w)
	}
}

// Without a cap the doubling reaches days, and a player who waits days is
// locked out in every sense that matters — which ADR-0007 forbids.
func TestTheWaitIsCapped(t *testing.T) {
	t.Parallel()

	c := newClock()
	policy := testPolicy()
	l := ratelimit.NewAt(policy, c.now)

	for range 40 {
		l.Failed("anna")
	}

	if got := l.Retry("anna"); got != policy.Max {
		t.Errorf("Retry() after forty failures = %s, want the cap %s", got, policy.Max)
	}
}

// The whole brake has to elapse on its own. This is the property that keeps
// it from rebuilding issue #70.
func TestTheWaitAlwaysRunsOut(t *testing.T) {
	t.Parallel()

	c := newClock()
	policy := testPolicy()
	l := ratelimit.NewAt(policy, c.now)

	for range 40 {
		l.Failed("anna")
	}
	if l.Retry("anna") == 0 {
		t.Fatal("forty failures cost nothing")
	}

	c.pass(policy.Max)
	if got := l.Retry("anna"); got != 0 {
		t.Errorf("Retry() after sitting out the cap = %s, want 0", got)
	}
}

func TestAQuietKeyIsForgotten(t *testing.T) {
	t.Parallel()

	c := newClock()
	policy := testPolicy()
	l := ratelimit.NewAt(policy, c.now)

	for range 10 {
		l.Failed("anna")
	}
	c.pass(policy.Forget)

	if got := l.Retry("anna"); got != 0 {
		t.Fatalf("Retry() after the forgetting period = %s, want 0", got)
	}

	// Forgotten means forgotten: the next failure starts from the free
	// attempts again, not from where the old count left off.
	l.Failed("anna")
	if got := l.Retry("anna"); got != 0 {
		t.Errorf("the first failure after being forgotten costs %s, want 0", got)
	}
}

func TestSuccessClearsTheCount(t *testing.T) {
	t.Parallel()

	c := newClock()
	l := ratelimit.NewAt(testPolicy(), c.now)

	for range 10 {
		l.Failed("anna")
	}
	l.Succeeded("anna")

	if got := l.Retry("anna"); got != 0 {
		t.Fatalf("Retry() after a success = %s, want 0", got)
	}
	l.Failed("anna")
	if got := l.Retry("anna"); got != 0 {
		t.Errorf("the first failure after a success costs %s, want 0", got)
	}
}

// One player's wrong guesses must not slow down anybody else. Otherwise a
// single person hammering the form takes the office out with them.
func TestKeysAreSeparate(t *testing.T) {
	t.Parallel()

	c := newClock()
	l := ratelimit.NewAt(testPolicy(), c.now)

	for range 10 {
		l.Failed("anna")
	}

	if l.Retry("anna") == 0 {
		t.Fatal("anna is not being slowed down")
	}
	if got := l.Retry("bodo"); got != 0 {
		t.Errorf("bodo waits %s for anna's guesses", got)
	}
}

// A guess a second against six digits is a million seconds; the point of the
// brake is that it is not that. This states the arithmetic the ADR argues
// from, so a later change to the policy has to face it.
func TestTheCapMakesGuessingHopeless(t *testing.T) {
	t.Parallel()

	c := newClock()
	policy := ratelimit.Policy{
		Free:   3,
		Step:   2 * time.Second,
		Max:    5 * time.Minute,
		Forget: time.Hour,
	}
	l := ratelimit.NewAt(policy, c.now)

	// Walk the brake up to its cap, counting the guesses it let through.
	guesses := 0
	for range 30 {
		l.Failed("anna")
		guesses++
		c.pass(l.Retry("anna"))
	}

	if got := l.Retry("anna"); got != 0 {
		t.Fatalf("the wait did not run out: %s", got)
	}

	// From here every further guess costs the cap. Six digits is 10^6.
	const pinSpace = 1_000_000
	atCap := time.Duration(pinSpace-guesses) * policy.Max
	if atCap < 24*time.Hour*365 {
		t.Errorf("working through six digits takes %s, which is not long enough", atCap)
	}
}

func TestConcurrentUseIsSafe(t *testing.T) {
	t.Parallel()

	l := ratelimit.New(testPolicy())

	var wg sync.WaitGroup
	for i := range 50 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			key := string(rune('a' + i%5))
			l.Failed(key)
			l.Retry(key)
			if i%7 == 0 {
				l.Succeeded(key)
			}
		}()
	}
	wg.Wait()
}
