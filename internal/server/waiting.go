package server

import (
	"strconv"
	"time"
)

// staleAfter is when a waiting result stops being normal.
//
// The measurement week put the median confirmation at 4.8 minutes, so almost
// anything still open the next day is stuck rather than slow — the two that
// prompted issue #159 had been open 17 and 22 hours. A full day is also the
// point where the label stops counting hours, so the emphasis and the wording
// change together instead of drawing two different lines.
const staleAfter = 24 * time.Hour

// waitedSince says how long a result has been waiting, coarsely, and whether
// that is long enough to be worth looking at.
//
// Coarse on purpose: the reader wants to know whether this is normal or
// stuck, not when it happened — they were there. Below an hour it says
// nothing at all, because "seit 2 Minuten" on the ordinary case is how a
// field trains its reader to skip it.
//
// Measured in elapsed time rather than calendar days, which is why it says
// "seit einem Tag" and not "seit gestern". Elapsed is what decides whether
// something is stuck; calendar is what the words "gestern" and "vorgestern"
// promise, and 40 hours is one of them about half the time. On a screen whose
// whole job is to be believed, a label the reader can contradict from memory
// costs more than a plainer one.
func waitedSince(now, playedAt time.Time) (label string, stale bool) {
	d := now.Sub(playedAt)

	switch {
	case d < time.Hour:
		return "", false
	case d < staleAfter:
		hours := int(d / time.Hour)
		if hours <= 1 {
			return "seit einer Stunde", false
		}
		return "seit " + strconv.Itoa(hours) + " Stunden", false
	}

	days := int(d / (24 * time.Hour))
	if days <= 1 {
		return "seit einem Tag", true
	}
	return "seit " + strconv.Itoa(days) + " Tagen", true
}
