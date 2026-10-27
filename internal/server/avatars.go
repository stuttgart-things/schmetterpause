package server

import (
	"context"
	"errors"
	"math/rand/v2"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/stuttgart-things/schmetterpause/internal/auth"
	"github.com/stuttgart-things/schmetterpause/internal/avatar"
	"github.com/stuttgart-things/schmetterpause/internal/domain"
	"github.com/stuttgart-things/schmetterpause/internal/templates"
	"github.com/stuttgart-things/schmetterpause/web"
)

// figures is the drawn parts of every player's figure (docs/adr/0029). Read
// once from the binary; avatar.MustLoad panics on a missing part, which the
// avatar tests reach first.
var figures = avatar.MustLoad(web.Static, "static/img/sthings")

// avatarOf is the figure a player wears: what they stored, with every part
// they did not store or that no longer exists filled from their default. A
// player who has never chosen is not an error, and neither is a database that
// cannot answer — the figure is decoration, and a page is worth showing
// without the right one.
func (s *Server) avatarOf(ctx context.Context, playerID uuid.UUID) avatar.Avatar {
	stored, err := s.store.Avatars().Get(ctx, playerID)
	switch {
	case errors.Is(err, domain.ErrNotFound):
		return avatar.Default(playerID.String())
	case err != nil:
		s.log.WarnContext(ctx, "loading the figure failed, showing the default",
			"player_id", playerID, "error", err)
		return avatar.Default(playerID.String())
	}
	return avatar.Fill(stored, playerID.String())
}

// pageFigure is a player's figure for beside a heading, and the class of its
// blade for the box around it.
//
// The class is the same paddle-n the box has carried since #64, now taken
// from the blade the figure wears rather than from the id. The figure sets
// --paddle itself; the class keeps the box saying which blade that is, which
// is what the pipeline's check of the start page and the profile reads.
func (s *Server) pageFigure(ctx context.Context, playerID uuid.UUID) (svg, blade string) {
	a := s.avatarOf(ctx, playerID)
	if i, ok := avatar.BladeOf(a); ok {
		blade = "paddle-" + strconv.Itoa(i)
	}
	return figures.Render(a), blade
}

// handleAvatar serves a player's figure with every face, for whatever stands
// next to a running match: the start page's live score and the Zählwerk's
// screen (docs/adr/0029). Which face shows is data-face on the element;
// ?face= sets it to begin with.
//
// Public, like the profile the figure is on anyway. Outside the session
// middleware, like /static: a figure is the same whoever asks, and the
// Zählwerk asks without a cookie.
func (s *Server) handleAvatar(w http.ResponseWriter, r *http.Request) {
	name, ok := strings.CutSuffix(r.PathValue("file"), ".svg")
	id, err := uuid.Parse(name)
	if !ok || err != nil {
		http.NotFound(w, r)
		return
	}
	if _, err := s.store.Players().ByID(r.Context(), id); err != nil {
		if !errors.Is(err, domain.ErrNotFound) {
			s.log.ErrorContext(r.Context(), "loading the player for a figure failed", "player_id", id, "error", err)
		}
		http.NotFound(w, r)
		return
	}
	// A minute: long enough that a page loading two figures and a reload do
	// not ask twice, short enough that a changed figure is on the next
	// match's screen.
	writeFigure(w, "public, max-age=60", figures.RenderLive(s.avatarOf(r.Context(), id), r.URL.Query().Get("face")))
}

// handleRandomAvatar serves a figure rolled for this request, for a match
// with nobody behind it: the Zählwerk's demo matches, Pong A against Pong B.
// Different on every call, as the Zählwerk's random blade has been.
func (s *Server) handleRandomAvatar(w http.ResponseWriter, r *http.Request) {
	a := avatar.Random(rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64()))) //nolint:gosec // decoration, not a secret
	writeFigure(w, "no-store", figures.RenderLive(a, r.URL.Query().Get("face")))
}

// writeFigure answers with one figure as an SVG document.
//
// The policy is for a browser that opens the file on its own: an SVG is a
// document that could carry script, and this one needs nothing but its own
// inline style. Inlined into a page, the header does not apply and the
// markup is what avatar.Render wrote, from palette values only.
func writeFigure(w http.ResponseWriter, cache, svg string) {
	h := w.Header()
	h.Set("Content-Type", "image/svg+xml")
	h.Set("Cache-Control", cache)
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'")
	_, _ = w.Write([]byte(svg))
}

// figureEditorView describes the editor with a checked, its preview rendered.
func figureEditorView(a avatar.Avatar) templates.FigureEditorView {
	colours := func(name, label string, palette []avatar.Colour, key string) templates.FigurePart {
		part := templates.FigurePart{Name: name, Label: label}
		for _, c := range palette {
			part.Options = append(part.Options, templates.FigureOption{
				Key: c.Key, Name: c.Name, Hex: c.Hex,
				SameAsFur: c.Key == avatar.SameAsFur,
				Checked:   c.Key == key,
			})
		}
		return part
	}
	bling := templates.FigurePart{Name: "bling", Label: "Schmuck"}
	for _, o := range avatar.Blings {
		bling.Options = append(bling.Options, templates.FigureOption{Key: o.Key, Name: o.Name, Checked: o.Key == a.Bling})
	}
	return templates.FigureEditorView{
		Preview: figures.Render(a),
		Parts: []templates.FigurePart{
			colours("fur", "Fell", avatar.Furs, a.Fur),
			colours("headdress", "Kopfschmuck", avatar.Headdresses, a.Headdress),
			colours("handle", "Griff", avatar.Handles, a.Handle),
			colours("blade", "Schläger", avatar.Blades, a.Blade),
			bling,
		},
	}
}

// figureFromForm reads the editor's fields. Validated by the caller: an
// unknown key from a form is a mistake to answer, not to paper over.
func figureFromForm(r *http.Request) avatar.Avatar {
	return avatar.Avatar{
		Fur:       r.FormValue("fur"),
		Headdress: r.FormValue("headdress"),
		Handle:    r.FormValue("handle"),
		Blade:     r.FormValue("blade"),
		Bling:     r.FormValue("bling"),
	}
}

// handleFigurePreview answers every change in the editor with the figure as
// it would look. Nothing is stored.
func (s *Server) handleFigurePreview(w http.ResponseWriter, r *http.Request) {
	a := figureFromForm(r)
	if err := avatar.Validate(a); err != nil {
		// Only a hand-made request gets here; the form offers nothing else.
		// The preview keeps what it showed.
		w.WriteHeader(http.StatusUnprocessableEntity)
		return
	}
	s.render(w, r, templates.FigurePreview(figures.Render(a)))
}

// handleSaveFigure stores the editor's figure for the reader, and nobody else:
// the player comes from the session, never from the form (docs/adr/0029,
// "Ändern kann jeder nur seine eigene Figur").
func (s *Server) handleSaveFigure(w http.ResponseWriter, r *http.Request) {
	self, _ := auth.PlayerID(r.Context())
	a := figureFromForm(r)
	if err := avatar.Validate(a); err != nil {
		view := figureEditorView(s.avatarOf(r.Context(), self))
		view.Error = "Das ließ sich nicht speichern. Lade die Seite neu und versuch es noch einmal."
		w.WriteHeader(http.StatusUnprocessableEntity)
		s.render(w, r, templates.FigureEditor(view))
		return
	}
	if err := s.store.Avatars().Put(r.Context(), self, a); err != nil {
		s.log.ErrorContext(r.Context(), "storing the figure failed", "player_id", self, "error", err)
		view := figureEditorView(a)
		view.Error = "Das hat gerade nicht geklappt. Versuch es noch einmal."
		w.WriteHeader(http.StatusInternalServerError)
		s.render(w, r, templates.FigureEditor(view))
		return
	}
	s.log.InfoContext(r.Context(), "figure saved", "player_id", self)
	s.renderFigureChange(w, r, self, func(v *templates.FigureEditorView) { v.Saved = true })
}

// handleResetFigure forgets the reader's figure, so they wear the default
// again. Pressing it twice is still reset.
func (s *Server) handleResetFigure(w http.ResponseWriter, r *http.Request) {
	self, _ := auth.PlayerID(r.Context())
	if err := s.store.Avatars().Delete(r.Context(), self); err != nil {
		s.log.ErrorContext(r.Context(), "resetting the figure failed", "player_id", self, "error", err)
		view := figureEditorView(s.avatarOf(r.Context(), self))
		view.Error = "Das hat gerade nicht geklappt. Versuch es noch einmal."
		w.WriteHeader(http.StatusInternalServerError)
		s.render(w, r, templates.FigureEditor(view))
		return
	}
	s.log.InfoContext(r.Context(), "figure reset", "player_id", self)
	s.renderFigureChange(w, r, self, func(v *templates.FigureEditorView) { v.Reset = true })
}

// renderFigureChange answers a save or a reset: the editor with what is now
// stored, and the figure beside the name out of band, so the page agrees with
// itself without a reload.
func (s *Server) renderFigureChange(w http.ResponseWriter, r *http.Request, self uuid.UUID, mark func(*templates.FigureEditorView)) {
	view := figureEditorView(s.avatarOf(r.Context(), self))
	mark(&view)
	s.render(w, r, templates.FigureEditor(view))
	svg, blade := s.pageFigure(r.Context(), self)
	s.render(w, r, templates.ProfileFigure(svg, blade, true))
}

// handleRollFigure fills the editor with a random figure. Not stored: "Würfeln"
// is a suggestion, and "Speichern" is still the one button that keeps it.
func (s *Server) handleRollFigure(w http.ResponseWriter, r *http.Request) {
	a := avatar.Random(rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64()))) //nolint:gosec // decoration, not a secret
	view := figureEditorView(a)
	view.Rolled = true
	s.render(w, r, templates.FigureEditor(view))
}
