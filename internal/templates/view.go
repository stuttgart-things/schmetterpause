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

// IndexView is everything the start page renders. Bundled rather than passed
// as four parameters so adding a section later does not touch every caller.
type IndexView struct {
	Session SessionView
	Players PlayerListView
	Match   MatchFormView
	Pending PendingListView
	// ShowMatch hides result entry from anyone who is not recognised yet —
	// a match has to be attributable to whoever reported it.
	ShowMatch bool
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
	// TTR is the current rating. Shown so a confirmation visibly does
	// something; games played and the win/loss record turn this into a real
	// ranking in AP6.
	TTR int
	// IsSelf marks the viewer, so the list answers "am I in here?" at a glance.
	IsSelf bool
}

// MaxSetRows is how many set rows the entry form offers. Best-of-seven is the
// longest mode, so seven rows can hold any legal result.
const MaxSetRows = 7

// MatchFormView drives the result entry form. Everything the player typed
// comes back on a rejection, so nobody re-enters a match they already typed.
type MatchFormView struct {
	Opponents   []OpponentOption
	BestOf      int
	PointsToWin int
	Sets        []SetInput
	// Error explains why the last attempt was rejected, in the words a
	// player can act on. Empty on a fresh form.
	Error string
}

// OpponentOption is one entry in the opponent picker.
type OpponentOption struct {
	ID          string
	DisplayName string
	Selected    bool
}

// SetInput holds a set as typed. Strings rather than numbers so that
// nonsense survives a rejection and can be shown back and corrected.
type SetInput struct {
	Home, Away string
}

// NewMatchFormView returns an empty form for the given opponents.
func NewMatchFormView(opponents []OpponentOption) MatchFormView {
	return MatchFormView{
		Opponents:   opponents,
		BestOf:      5,
		PointsToWin: 11,
		Sets:        make([]SetInput, MaxSetRows),
	}
}

// MatchRecordedView confirms a recorded result.
type MatchRecordedView struct {
	OpponentName string
	// OwnSets and OpponentSets are from the reporting player's side, because
	// that is who is reading it.
	OwnSets, OpponentSets int
	Won                   bool
}

// PendingListView is the set of results waiting on the viewer.
type PendingListView struct {
	Matches []PendingMatchView
}

// PendingMatchView is one result awaiting a decision, seen from the side of
// the player who has to make it — they read "3:1 für dich", not "home 3:1".
type PendingMatchView struct {
	ID           string
	ReporterName string
	OwnSets      int
	OpponentSets int
	Sets         []SetScore
	// Won is what the reported result claims for the viewer.
	Won bool
}

// SetScore is one set, again from the viewer's side.
type SetScore struct {
	Own, Opponent int
}

// SettledView confirms that a result now counts, and by how much.
type SettledView struct {
	// ID identifies the list entry this replaces.
	ID           string
	OpponentName string
	Won          bool
	OwnSets      int
	OpponentSets int
	// Delta is the viewer's rating change, NewTTR where it left them.
	Delta  int
	NewTTR int
}

// DisputedView reports a contested result.
type DisputedView struct {
	ID           string
	ReporterName string
}
