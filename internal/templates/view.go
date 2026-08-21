// Package templates holds the templ templates and their view models.
//
// Handlers return HTML fragments, not JSON APIs (CLAUDE.md, "Konventionen").
// The generated *_templ.go files are checked in; the pipeline's lint step
// verifies they match the current *.templ sources.
package templates

// generate regenerates the *_templ.go files. templ is pinned as a Go tool in
// go.mod so that the same version runs locally and in the pipeline.
//
// Always regenerate through `go generate ./...` rather than calling templ
// from the repository root: templ records the source path it was given in the
// generated code, so the two invocations produce different output and the
// pipeline's up-to-date check fails.
//
//go:generate go tool templ generate

// StatusView is the model behind the status fragment on the start page. In
// the scaffolding it serves as proof that template, HTMX and repository work
// together.
type StatusView struct {
	// Players is the number of players on record.
	Players int
	// DatabaseReachable reports whether the database answers a ping.
	DatabaseReachable bool
	// Version is the version of the running binary.
	Version string
}

// SessionView drives the join form and the signed-in notice. Both render into
// the same #session region, so the form can replace itself with the result.
type SessionView struct {
	// DisplayName is set once a player is recognised.
	DisplayName string
	// Name is the value put back into the form after a rejected attempt, so
	// nobody has to type their name twice.
	Name string
	// Error explains why the last attempt was rejected. Empty on success.
	Error string
}

// SignedIn reports whether a player is recognised.
func (v SessionView) SignedIn() bool { return v.DisplayName != "" }

// PlayerListView is the list of players. Not a ranking — that is AP6.
type PlayerListView struct {
	Players []PlayerListEntry
}

// PlayerListEntry is one player in the list.
type PlayerListEntry struct {
	DisplayName string
	// IsSelf marks the viewer, so the list answers "am I in here?" at a glance.
	IsSelf bool
}
