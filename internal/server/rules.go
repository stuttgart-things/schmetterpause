package server

import (
	"net/http"

	"github.com/stuttgart-things/schmetterpause/internal/match"
	"github.com/stuttgart-things/schmetterpause/internal/templates"
)

// handleRulesSheet renders the house rules, made to be printed and taped up
// next to the QR sheet.
//
// Rendered rather than served as a static file, because two of the rules are
// statements about the target score the entry form offers, and the template
// derives them from the same helpers the form uses.
func (s *Server) handleRulesSheet(w http.ResponseWriter, r *http.Request) {
	s.render(w, r, templates.RulesSheet(templates.RulesSheetView{
		Header: s.headerView(r.Context()),
		// The mode a set is played in over a break, which is the mode the
		// sheet on the wall is about.
		PointsToWin: match.PointsToEleven,
	}))
}
