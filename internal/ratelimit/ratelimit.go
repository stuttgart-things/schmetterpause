// Package ratelimit slows down guessing at a shared secret.
//
// docs/adr/0007 makes this a shipping condition rather than follow-up work:
// six digits is a million values, and without a brake the length is the only
// thing between somebody and another player — through the same door the
// recovery code uses.
//
// It has one hard requirement beside that, and the two pull against each
// other: **the brake must never lock a player out for good.** A limit that
// did would rebuild issue #70 by another route, which is the thing all of
// this exists to remove. So every wait here elapses on its own, a quiet key
// is forgotten entirely, and a success clears the count.
//
// In memory, per process. No Redis (invariant 3), and none is needed: the
// application is one binary with one replica during the measurement, and a
// restart forgetting who was being slowed down errs towards letting people
// in — which is the right way for this to fail.
package ratelimit

import (
	"sync"
	"time"
)

// Policy is how hard one dimension pushes back.
type Policy struct {
	// Free is how many failures pass before any wait at all. Somebody
	// mistyping a sixteen-character code twice is not an attack, and a
	// stopwatch on the second attempt would punish exactly the person this
	// whole way back was built for.
	Free int
	// Step is the wait after the first failure past Free. It doubles from
	// there.
	Step time.Duration
	// Max caps the wait. Without a cap the doubling reaches days, and a
	// player who has to wait days is locked out in every sense that matters.
	Max time.Duration
	// Forget is how long a key keeps its failures after the last one. Past
	// it the key is a stranger again — which is the other half of "never
	// permanent".
	Forget time.Duration
}

// wait is how long a key with this many consecutive failures has to sit out.
func (p Policy) wait(failures int) time.Duration {
	over := failures - p.Free
	if over <= 0 {
		return 0
	}
	// Shifted rather than looped, and clamped before the shift can run off
	// the end of an int64 and come back negative.
	if over > 62 {
		return p.Max
	}
	d := p.Step << (over - 1)
	if d <= 0 || d > p.Max {
		return p.Max
	}
	return d
}

// sweepAbove is the map size past which a failed attempt also drops the keys
// nobody has touched in a while. Below it, walking the map costs more than
// the handful of entries it would free.
const sweepAbove = 1024

// Limiter counts failed attempts per key under one policy.
//
// One dimension per Limiter. The sign-in path holds two — one keyed on the
// player, one on the address — because a limit on only one of them is not a
// limit: per player alone, an attacker walks the roster; per address alone,
// a second phone starts over.
type Limiter struct {
	policy Policy
	now    func() time.Time

	mu   sync.Mutex
	keys map[string]*attempts
}

type attempts struct {
	failures int
	// last is when the most recent failure landed, and what Forget measures.
	last time.Time
	// until is when the next attempt may go ahead.
	until time.Time
}

// New builds a limiter on the wall clock.
func New(p Policy) *Limiter { return NewAt(p, time.Now) }

// NewAt builds a limiter that reads the time from now.
//
// Exported for tests, which have to be able to let an hour pass without
// taking an hour. A limiter whose waits could only be tested by waiting them
// out would be a limiter nobody tests.
func NewAt(p Policy, now func() time.Time) *Limiter {
	return &Limiter{policy: p, now: now, keys: map[string]*attempts{}}
}

// Retry reports how long this key still has to sit out. Zero means go ahead.
func (l *Limiter) Retry(key string) time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()

	a, ok := l.keys[key]
	if !ok {
		return 0
	}

	now := l.now()
	if now.Sub(a.last) >= l.policy.Forget {
		delete(l.keys, key)
		return 0
	}
	if left := a.until.Sub(now); left > 0 {
		return left
	}
	return 0
}

// Failed records an attempt that did not work out, and lengthens the wait.
func (l *Limiter) Failed(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()

	a, ok := l.keys[key]
	if !ok || now.Sub(a.last) >= l.policy.Forget {
		a = &attempts{}
		l.keys[key] = a
	}

	a.failures++
	a.last = now
	a.until = now.Add(l.policy.wait(a.failures))

	l.sweep(now)
}

// Succeeded clears the count.
//
// Whoever proved who they are is not the person the brake was for, and
// leaving their count standing would slow down their next honest mistake for
// something that is already settled.
func (l *Limiter) Succeeded(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	delete(l.keys, key)
}

// sweep drops keys nobody has touched within the forgetting period, so a long
// evening of wrong guesses does not leave the map holding every address that
// ever tried.
func (l *Limiter) sweep(now time.Time) {
	if len(l.keys) <= sweepAbove {
		return
	}
	for k, a := range l.keys {
		if now.Sub(a.last) >= l.policy.Forget {
			delete(l.keys, k)
		}
	}
}
