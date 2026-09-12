package server

import (
	"net/http"

	"github.com/stuttgart-things/schmetterpause/internal/templates"
)

// handleInfo serves the page the status lives on.
//
// The numbers come from the fragment the page loads rather than from here: one
// place that knows how to ask the database whether it is there, instead of two
// that can disagree about it.
func (s *Server) handleInfo(w http.ResponseWriter, r *http.Request) {
	s.render(w, r, templates.Info(templates.StatusView{
		Header:     s.headerView(r.Context()),
		Version:    s.build.Version,
		CommitTime: s.build.CommitTime,
	}))
}

// handleVersion answers with the version this binary was built as, and
// nothing else.
//
// The same string /info renders, in a form something other than a person can
// read. That is the whole difference, and it is the point: /info is HTML from
// a template, so anything comparing "what is running" against "what was
// released" would be parsing a page. One line of plain text is a `curl` in a
// probe, a body regex in a blackbox exporter, or a string comparison in a
// scheduled job — whichever of those the comparison ends up living in
// (issue #229).
//
// The version alone, deliberately. The commit time is on /info for whoever
// wants to know how old this is; a second field here would invite parsing the
// one thing that exists so it does not have to be parsed.
//
// Registered beside /healthz and /readyz rather than inside the page routes:
// the HTTPRoute is a single PathPrefix "/" (kcl/httproute.k), so a new public
// path is a decision to take on purpose rather than one inherited by being
// added anywhere at all. It says no more than the start page already does to
// anybody who opens it.
func (s *Server) handleVersion(w http.ResponseWriter, _ *http.Request) {
	writePlain(w, http.StatusOK, s.build.Version)
}

// handleStatusFragment serves the status fragment loaded by HTMX.
//
// It is the proof that handler, repository, database and template work
// together — exactly the path the pipeline's verify step checks against the
// built image, which is why the markup it asserts on should stay stable.
func (s *Server) handleStatusFragment(w http.ResponseWriter, r *http.Request) {
	view := templates.StatusView{
		Version:    s.build.Version,
		CommitTime: s.build.CommitTime,
	}

	if err := s.store.Ping(r.Context()); err != nil {
		s.log.WarnContext(r.Context(), "status: database unreachable", "error", err)
		s.render(w, r, templates.Status(view))
		return
	}
	view.DatabaseReachable = true

	count, err := s.store.Players().Count(r.Context())
	if err != nil {
		s.log.ErrorContext(r.Context(), "status: counting players failed", "error", err)
		// User-facing text stays German; see CLAUDE.md.
		http.Error(w, "Status nicht ermittelbar", http.StatusInternalServerError)
		return
	}
	view.Players = count

	s.render(w, r, templates.Status(view))
}
