// Package templates holds the templ templates and their view models.
//
// Handlers return HTML fragments, not JSON APIs (CLAUDE.md, "Konventionen").
// The generated *_templ.go files are checked in; the pipeline's lint step
// verifies they match the current *.templ sources.
package templates

import (
	"hash/fnv"
	"strconv"

	"github.com/stuttgart-things/schmetterpause/internal/credential"
	"github.com/stuttgart-things/schmetterpause/internal/match"
)

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

// Ready is what /readyz would answer. Readiness is defined by the database
// check and nothing else, so the two cannot drift apart by being stated in
// two places — this reads the same field the probe does.
func (v StatusView) Ready() bool { return v.DatabaseReachable }

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
	// UndoID is the match the note is about, when it can still be taken
	// back. Empty on a fresh page: the offer belongs to the answer of the
	// entry that produced it, not to the page in general.
	UndoID string
	// Error explains a refusal. Empty otherwise.
	Error string
	// IssuedCode is a recovery code just made for somebody else, in the
	// clear, and shown once. IssuedFor names whose it is, because the person
	// reading the screen is not the person it belongs to — which is the whole
	// difference between this display and the one after joining.
	IssuedCode string
	IssuedFor  string
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
	// IsAdmin shows the link to /admin. Only to somebody who holds the flag:
	// a link everybody sees to a page only some may open is a link that
	// mostly produces a refusal.
	IsAdmin bool
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
	// PlayerID is what the mascot's blade colour is derived from. Empty
	// before anybody is recognised, which leaves the blade red.
	PlayerID string
	// Name is the value put back into the form after a rejected attempt, so
	// nobody has to type their name twice.
	Name string
	// RecoveryCode is the freshly issued code, in the clear, and it is set
	// in exactly one response: the one that created the player. It is shown
	// once and never stored anywhere it could be shown again — the server
	// keeps only a hash (docs/adr/0006).
	RecoveryCode string
	// Error explains why the last attempt was rejected. Empty on success.
	Error string
}

// PINFormView drives the form that sets a PIN.
type PINFormView struct {
	// Set reports whether this player already has one. Only the wording
	// depends on it — setting and replacing are the same write.
	Set bool
	// Done marks the response that follows a successful change. What was
	// typed is never handed back, so there is nothing else to say.
	Done bool
	// Error explains a refusal.
	Error string
}

// Heading labels the field.
func (v PINFormView) Heading() string {
	if v.Set {
		return "Neue PIN"
	}
	return "PIN, wenn du magst"
}

// Action labels the button.
func (v PINFormView) Action() string {
	if v.Set {
		return "PIN ändern"
	}
	return "PIN setzen"
}

// The PIN bounds as strings, for the field attributes and the sentence above
// them. Read from the credential package rather than written out here, so the
// form, the browser and the server cannot disagree about what is allowed.
var (
	MinPIN = strconv.Itoa(credential.MinPINLength)
	MaxPIN = strconv.Itoa(credential.MaxPINLength)
)

// SignOutView is the button that makes this browser a stranger again.
type SignOutView struct {
	// HasPIN decides which warning is shown. Somebody without one has a
	// single way back and should be told so before they press it.
	HasPIN bool
}

// RecoveryCardView is the recovery code on somebody's own profile.
type RecoveryCardView struct {
	// Code is a freshly issued one, in the clear, set only in the response
	// that produced it. Empty on the ordinary render, which offers to make
	// one rather than showing the one that exists — the server holds only a
	// hash and could not show it if it wanted to.
	Code string
	// Error explains a refusal.
	Error string
}

// SignInView drives the sign-in form: who is signing in, and why the last
// attempt was refused.
type SignInView struct {
	// Players is everybody, by name. Publishing the list costs nothing —
	// the ranking, /matches and the sheet on the wall all name everybody —
	// and with a salt per row there is no way to find a credential from the
	// secret alone, so the name has to come first (docs/adr/0007).
	//
	// OpponentOption because a picker is a picker; the kiosk reuses it for
	// its two selects for the same reason.
	Players []OpponentOption
	// Error explains why the last attempt was refused, in one wording for
	// every reason. Which half was wrong is deliberately not said.
	Error string
}

// SignedIn reports whether a player is recognised.
func (v SessionView) SignedIn() bool { return v.DisplayName != "" }

// anySelected reports whether the picker already stands on somebody, which
// is the case after a refused attempt. Without it the placeholder would take
// the selection back and the name would have to be found twice.
func (v SignInView) anySelected() bool {
	for _, p := range v.Players {
		if p.Selected {
			return true
		}
	}
	return false
}

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

// BestOfOptions is the mode picker's choices, shortest first. It mirrors
// allowedBestOf in the match package, which mirrors the schema constraint.
var BestOfOptions = []int{1, 3, 5, 7}

// BestOfLabel names a mode the way it gets said at the table. "Best of 1" is
// how the number reads and not what anybody calls a single set, so the one
// case gets its own words (issue #114).
func BestOfLabel(bestOf int) string {
	if bestOf == 1 {
		return "Ein Satz"
	}
	return "Best of " + strconv.Itoa(bestOf)
}

// ServeEvery is how many points a player serves in a row before the service
// changes: two in a set to eleven, five in a set to twenty-one.
//
// Not a constant, because the form offers both modes and the answer is
// different in each. "Always two" is the rule most people know, and it is the
// wrong one for the twenty-one mode the picker also offers.
func ServeEvery(pointsToWin int) int {
	if pointsToWin >= 21 {
		return 5
	}
	return 2
}

// DeuceFrom is the score from which a set no longer ends at the target.
//
// This is why the boxes carry no maximum: at eleven, 12:10 and 13:11 are
// ordinary results, and a form that refused them would be wrong more often
// than the one that accepts 15:8 and has the server explain itself.
func DeuceFrom(pointsToWin int) int { return pointsToWin - 1 }

// SetsView is what /fragments/sets needs: which form asked, in which mode,
// with what already typed into it, and whose points go in which box.
type SetsView struct {
	Prefix      string
	BestOf      int
	PointsToWin int
	Sets        []SetInput
	// Picker carries the opposite player select back with the response, so
	// it can drop whoever this one just took. Nil everywhere but the kiosk.
	Picker *KioskPicker
	// HomeLabel and AwayLabel head the two score columns. Two boxes with a
	// colon between them do not say which side is which, and at the table
	// nobody stops to work it out — they type, and the wrong player wins.
	HomeLabel, AwayLabel string
}

// KioskPicker is one of the kiosk's two player selects, re-rendered out of
// band with the other side's choice removed from it.
type KioskPicker struct {
	ID      string
	Name    string
	Players []OpponentOption
}

// scoreValue is what a score box shows: what was typed, or zero.
//
// A default rather than an empty box, so the slider beside it has something
// to point at from the start and nobody has to type a zero for a set somebody
// lost to nil. A row that stays at 0:0 counts as not played — table tennis
// has no draws, so the two cannot be confused.
func scoreValue(value string) string {
	if value == "" {
		return "0"
	}
	return value
}

// SideDefaults are the column headings before anybody is picked.
const (
	SideHome     = "Spieler"
	SideAway     = "Gegner"
	SideYourself = "Du"
)

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

// AwayLabel is the name of the chosen opponent, or the generic word before
// anybody is chosen.
func (v MatchFormView) AwayLabel() string {
	for _, o := range v.Opponents {
		if o.Selected {
			return o.DisplayName
		}
	}
	return SideAway
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
		BestOf:      match.DefaultBestOf,
		PointsToWin: match.PointsToEleven,
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

// AnyRanked reports whether a single confirmed match has been played. Until
// then the table lists who is in and says so, rather than presenting a
// placement nobody earned.
func (v StandingsView) AnyRanked() bool {
	for _, row := range v.Rows {
		if row.Ranked() {
			return true
		}
	}
	return false
}

// rankClass names the chip a rank is drawn in.
//
// Only the podium is marked, and only first place is filled — in the mascot's
// orange, which is the one place the logo colour reaches into the interface.
// Everything from fourth place down is a bare number, because a rank column
// in five colours reads louder than the ratings beside it.
func rankClass(rank int, shared bool) string {
	class := "rank"
	switch {
	case rank == 0:
		return class + " rank-none"
	case rank <= 3:
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

// Ranked reports whether this row holds a position at all.
func (r StandingRow) Ranked() bool { return r.Rank > 0 }

// ProfileView is one player's page.
type ProfileView struct {
	Header HeaderView
	// ID is whose page this is, and what their blade colour comes from.
	ID          string
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
	// IsSelf marks the reader's own page, which is the only one carrying the
	// access section. Somebody else's PIN is none of their business, and the
	// page is otherwise public.
	IsSelf bool
	// HasPIN changes the wording from setting one to replacing one.
	HasPIN bool
}

// paddleColours is how many blade colours there are. Named rather than
// inlined because the count is part of the mapping: change it and every player
// gets a different colour than they had yesterday.
const paddleColours = 7

// The pages that belong to nobody rather than to a player: the kiosk stands
// at the table, the list is everybody's, and the sheet goes on a wall. They
// get a colour too, but a fixed one — a blade that comes up different on every
// reload reads as something broken rather than as decoration.
//
// These three are the widest-apart triple in the palette, no pair closer than
// dE 73.2, so the pages are told apart at a glance. Which page got which is
// arbitrary; that it never changes is not.
const (
	PaddleKiosk     = "paddle-0" // Hellblau
	PaddleMatchList = "paddle-3" // Moosgrün
	PaddleSheet     = "paddle-4" // Pink
)

// PaddleClass is the blade colour a player's mascot carries, as a class the
// stylesheet resolves to a --paddle value.
//
// Derived from the id rather than stored: no field, no picker, no migration,
// and the same colour on every device and after every restart. The cost is
// that nobody can choose theirs, and that in a group of seven two players will
// already share one. Which is why the colour is never the only thing telling
// them apart — the name is always beside it, and the sheet at /qr prints in
// black and white.
//
// Empty for a browser nobody is recognised in: no player, no colour, and the
// blade keeps the red it is drawn in.
func PaddleClass(playerID string) string {
	if playerID == "" {
		return ""
	}
	// FNV-1a: stable across builds and machines, which a map iteration or a
	// pointer would not be.
	h := fnv.New32a()
	_, _ = h.Write([]byte(playerID))
	return "paddle-" + strconv.Itoa(int(h.Sum32()%paddleColours))
}

// AdminView is the page that answers who may act for other people.
type AdminView struct {
	Header HeaderView
	People []AdminPerson
	// Kiosks is which machines are unlocked right now. Issue #77 filed
	// exactly this question, and the old derived cookie could not answer it:
	// it was the same value in every browser that had ever seen the token.
	Kiosks []KioskGrantView
}

// KioskGrantView is one unlocked machine.
type KioskGrantView struct {
	ID string
	// UserAgent is what the browser said it was. A label so the row reads as
	// a machine rather than as an identifier — never treated as identity.
	UserAgent string
	Unlocked  string
	LastSeen  string
	Expires   string
}

// Label is what to call this machine in a list. The user agent when there is
// one, and otherwise something that is at least not empty.
func (v KioskGrantView) Label() string {
	if v.UserAgent == "" {
		return "Unbekanntes Gerät"
	}
	return v.UserAgent
}

// AdminPerson is one holder of the flag.
type AdminPerson struct {
	ID          string
	DisplayName string
	// IsSelf marks the reader, so somebody checking whether they have it
	// does not have to look for their own name.
	IsSelf bool
}

// MatchListView is every match the office has played, newest first.
type MatchListView struct {
	Header  HeaderView
	Matches []MatchListRow
	// Limit is how many rows the list asks for at most. Truncated says the
	// answer hit it — a capped list that does not say so reads as the whole
	// history, and this one will not be the whole history forever.
	Limit     int
	Truncated bool
}

// MatchListRow is one match, read from the winner's side.
//
// Winner first rather than home first: whoever entered the result decided
// which side "home" was, which is an artefact of the form and not of the
// evening. Read winner-first, every row is a sentence — "Anna beat Bodo 2:0,
// and it was worth eight points".
type MatchListRow struct {
	PlayedAt   string
	WinnerName string
	LoserName  string
	WinnerID   string
	LoserID    string
	WinnerSets int
	LoserSets  int
	// Sets are the individual scores, also from the winner's side.
	Sets []SetScore
	// Delta is what the win was worth, and HasDelta is false for anything
	// that has not been settled. For a single match the two players' changes
	// are equal and opposite, so the winner's number says it all.
	Delta    int
	HasDelta bool
	Pending  bool
	Disputed bool
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
