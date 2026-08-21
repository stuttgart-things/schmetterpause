package server

import (
	"net/http"
	"strings"

	"github.com/stuttgart-things/schmetterpause/internal/qr"
	"github.com/stuttgart-things/schmetterpause/internal/templates"
)

// entryFragment is where a scan lands. The start page already carries result
// entry; the fragment only decides where the browser opens it.
//
// It is the whole of AP7's "jump straight into result entry". A separate lean
// page would be a second rendering of the same form, and the sections above
// the form on the start page are the ones somebody reads rather than uses.
// Before anyone is recognised the anchor does not exist, the browser stays at
// the top, and the join form is the first thing on the page — which is the
// right order for a first scan.
const entryFragment = "#match"

// handleQRSheet renders the sheet that goes up next to the table.
func (s *Server) handleQRSheet(w http.ResponseWriter, r *http.Request) {
	target := s.publicBaseURL(r) + "/" + entryFragment

	code, err := qr.Render(target)
	if err != nil {
		s.log.ErrorContext(r.Context(), "rendering the QR code failed", "target", target, "error", err)
		http.Error(w, "QR-Code nicht verfügbar", http.StatusInternalServerError)
		return
	}

	s.render(w, r, templates.QRSheet(templates.QRSheetView{
		Target: target,
		Path:   code.Path,
		Size:   code.Size,
	}))
}

// publicBaseURL is the address a phone has to reach, which is not necessarily
// the one this request arrived at.
//
// A QR code holds an absolute URL, so the sheet cannot be a static asset
// baked at build time without hardcoding one host — which invariant 2 rules
// out. It is derived per request instead, and SP_PUBLIC_BASE_URL overrides it
// where the request cannot be believed.
//
// The scheme comes from X-Forwarded-Proto where the connection itself is not
// encrypted, because every deployment target of this image terminates TLS in
// front of the process. The host comes from the request and never from
// X-Forwarded-Host: a proxy passes the Host header through, and honouring the
// header would let a caller decide what the code points at.
func (s *Server) publicBaseURL(r *http.Request) string {
	if s.cfg.PublicBaseURL != "" {
		return s.cfg.PublicBaseURL
	}

	scheme := "http"
	if r.TLS != nil || strings.EqualFold(forwardedProto(r), "https") {
		scheme = "https"
	}
	return scheme + "://" + r.Host
}

// forwardedProto reads the first entry of X-Forwarded-Proto. Several proxies
// in a row append to the header, and the first entry is the one that faced
// the client.
func forwardedProto(r *http.Request) string {
	proto := r.Header.Get("X-Forwarded-Proto")
	if i := strings.IndexByte(proto, ','); i >= 0 {
		proto = proto[:i]
	}
	return strings.TrimSpace(proto)
}
