package server

import "time"

// WaitInWords exposes the wording helper to the package's external tests.
// The German it produces is read by somebody who has just been refused, and
// a test that has to go through an HTTP round trip to check a plural is a
// test nobody writes.
func WaitInWords(d time.Duration) string { return waitInWords(d) }
