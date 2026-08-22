// Package templates holds the templ templates and their view models.
//
// Handlers return HTML fragments, not JSON APIs (CLAUDE.md, "Konventionen").
// The generated *_templ.go files are checked in; the pipeline's lint step
// verifies they match the current *.templ sources.
package templates

import "strconv"

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

// KioskView is the page one machine at the table works from.
//
// Sets and mode are not handed back after a rejection the way result entry
// does it: a scorekeeper is holding a sheet and reading from it, so the fix
// for a mistyped number is to type it again, not to hunt for what changed.
type KioskView struct {
	Players     []OpponentOption
	BestOf      int
	PointsToWin int
	Sets        []SetInput
	Standings   StandingsView
	// Note reports what just happened, in the words somebody at the table
	// would use: "Anna schlägt Bodo 3:1."
	Note string
	// Error explains a refusal. Empty otherwise.
	Error string
}

// HeaderView is the state in the top bar: who this is, and how much is
// waiting for them.
type HeaderView struct {
	DisplayName string
	// ProfileURL is where the name leads. The rating is the thing people
	// look up about themselves, so it leads to their own profile.
	ProfileURL string
	// Pending is how many results wait on this player.
	Pending int
}

// SignedIn reports whether anybody is recognised.
func (v HeaderView) SignedIn() bool { return v.DisplayName != "" }

// maxBadgeCount is where the badge stops counting. Past it the number stops
// being information and starts being a layout problem.
const maxBadgeCount = 9

func pendingCount(n int) string {
	if n > maxBadgeCount {
		return strconv.Itoa(maxBadgeCount) + "+"
	}
	return strconv.Itoa(n)
}

func pendingLabel(n int) string {
	if n == 1 {
		return "1 Ergebnis wartet auf deine Bestätigung"
	}
	return strconv.Itoa(n) + " Ergebnisse warten auf deine Bestätigung"
}

// QRSheetView is the printable sheet.
//
// The geometry is computed before the template sees it: a template is the
// wrong place to walk a module grid.
type QRSheetView struct {
	Header HeaderView
	// Target is the absolute URL the code encodes. It is printed as text as
	// well, so the sheet still works for a phone whose camera will not scan.
	Target string
	// Path is SVG path data, one unit per module.
	Path string
	// Size is the viewBox side length in modules, quiet zone included.
	Size int
}

// IndexView is everything the start page renders. Bundled rather than passed
// as four parameters so adding a section later does not touch every caller.
type IndexView struct {
	Header    HeaderView
	Session   SessionView
	Standings StandingsView
	Match     MatchFormView
	Pending   PendingListView
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

// MaxSetRows is how many set rows a submitted form can carry. Best-of-seven
// is the longest mode, so seven rows can hold any legal result. What a form
// *shows* is SetRows — a shorter mode shows fewer.
const MaxSetRows = 7

// SetRows is the rows a mode actually needs. Best-of-three has no fourth set,
// and four empty boxes underneath it are four chances to type into the wrong
// one.
//
// It clamps rather than pads: a best_of that arrived from somewhere other than
// the picker still renders, it just renders everything.
func SetRows(sets []SetInput, bestOf int) []SetInput {
	if bestOf <= 0 || bestOf > len(sets) {
		return sets
	}
	return sets[:bestOf]
}

// DeuceFrom is the score from which a set no longer ends at the target.
//
// This is why the boxes carry no maximum: at eleven, 12:10 and 13:11 are
// ordinary results, and a form that refused them would be wrong more often
// than the one that accepts 15:8 and has the server explain itself.
func DeuceFrom(pointsToWin int) int { return pointsToWin - 1 }

// SetsView is what /fragments/sets needs: which form asked, in which mode,
// with what already typed into it.
type SetsView struct {
	Prefix      string
	BestOf      int
	PointsToWin int
	Sets        []SetInput
}

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
//
// It covers both states an entry can be in, because they replace each other
// in place: a reported result waiting for yes or no, and a contested one
// waiting for a correction.
type PendingMatchView struct {
	ID           string
	ReporterName string
	// OpponentName is the other participant. On a reported result that is
	// the reporter; on a contested one it need not be, because either side
	// may have put the wrong number in.
	OpponentName string
	OwnSets      int
	OpponentSets int
	Sets         []SetScore
	// Won is what the reported result claims for the viewer.
	Won bool

	// Disputed switches the entry to the correction form.
	Disputed bool
	// BestOf, PointsToWin and Inputs pre-fill that form with what was
	// reported, from the viewer's side.
	BestOf      int
	PointsToWin int
	Inputs      []SetInput
	// Error explains why a correction was rejected. Empty otherwise.
	Error string
}

// CorrectedView reports a corrected result on its way back to the opponent.
type CorrectedView struct {
	ID           string
	OpponentName string
	// OwnSets and OpponentSets are from the correcting player's side.
	OwnSets, OpponentSets int
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

// StandingsView is the ranking.
type StandingsView struct {
	Rows []StandingRow
}

// rankClass names the chip a rank is drawn in. Only the first five positions
// carry a colour: five is a ladder, twelve is a fruit bowl.
func rankClass(rank int, shared bool) string {
	class := "rank"
	if rank >= 1 && rank <= 5 {
		class += " rank-" + strconv.Itoa(rank)
	}
	if shared {
		class += " shared"
	}
	return class
}

// StandingRow is one line of the ranking.
type StandingRow struct {
	ID   string
	Rank int
	// Shared marks a rank more than one player holds, so two identical
	// numbers do not have to explain themselves.
	Shared      bool
	DisplayName string
	TTR         int
	Played      int
	Won, Lost   int
	IsSelf      bool
}

// ProfileView is one player's page.
type ProfileView struct {
	Header      HeaderView
	DisplayName string
	TTR         int
	Rank        int
	Shared      bool
	Played      int
	Won, Lost   int
	// Delta is the last rating change; HasDelta is false before the first
	// confirmed match, where "±0" would claim something that never happened.
	Delta    int
	HasDelta bool
	Spark    SparkView
	Matches  []ProfileMatchView
}

// SparkView is the rating history as ready-to-draw geometry. The arithmetic
// happens in Go: a template is the wrong place to compute coordinates.
type SparkView struct {
	// Show is false with fewer than two points, where a line would be a dot
	// pretending to be a trend.
	Show bool
	// Points is the polyline, "x,y x,y …" in the viewBox.
	Points string
	// LastX and LastY mark the current rating, drawn in the accent while the
	// line stays recessive.
	LastX, LastY string
	// Width and Height are the viewBox.
	Width, Height int
	// Low and High label the range. The baseline is not zero — ratings sit
	// around 1000, and a zero baseline would flatten every match into noise —
	// so the range has to be stated rather than assumed.
	Low, High int
}

// ProfileMatchView is one match on a profile, from that player's side.
type ProfileMatchView struct {
	PlayedAt     string
	OpponentName string
	Won          bool
	OwnSets      int
	OpponentSets int
	Sets         []SetScore
	// Pending and Disputed mark a result that does not count yet, or at all.
	Pending  bool
	Disputed bool
	Delta    int
	HasDelta bool
}
