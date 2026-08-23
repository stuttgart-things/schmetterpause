package server

import (
	"io/fs"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/stuttgart-things/schmetterpause/web"
)

// staticHandler serves the embedded assets under /static/.
func staticHandler() http.Handler {
	assets, err := fs.Sub(web.Static, "static")
	if err != nil {
		// This can only break if the embed directory is missing, which means
		// the binary was built wrong and should say so immediately.
		panic("embedded assets not readable: " + err.Error())
	}

	fileServer := http.FileServerFS(assets)

	return http.StripPrefix("/static/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", cacheControl(r.URL.Path))
		fileServer.ServeHTTP(w, r)
	}))
}

// cacheControl decides how long an asset may be kept, and the answer depends
// on whether the file has to agree with the page that loaded it.
//
// The stylesheet and the script do. A deployment replaces the HTML instantly
// and the assets not at all, so for up to an hour a browser can hold the old
// script against the new markup — which looks exactly like a feature that was
// built and does not work. That happened: sliders drawn by new HTML, wired by
// a script that predates them, drawn in the browser's default colour because
// the stylesheet was stale too.
//
// Fonts and images do not have that problem. They are referenced by name, a
// stale one is cosmetic, and they are the assets worth caching — 84 kB of
// fonts and 130 kB of images against 20 kB of CSS and script.
//
// The proper fix is a content hash in the filename. This is the smaller one,
// and it is the one that is right on a laptop somebody redeploys mid-evening.
func cacheControl(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".css", ".js":
		// Not "no-store": the browser may keep it, it just has to ask.
		return "no-cache"
	default:
		return "public, max-age=3600"
	}
}

// faviconHandler answers the request every browser makes on its own with the
// same 32 px mark the pages link. A PNG under a .ico name is what the rest of
// the web does too; browsers go by the bytes, not by the extension.
func faviconHandler() http.Handler {
	icon, err := web.Static.ReadFile("static/img/mark-32.png")
	if err != nil {
		// Same reasoning as above: a missing asset is a broken build, and it
		// should be loud at startup rather than quiet on the first request.
		panic("embedded favicon not readable: " + err.Error())
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Cache-Control", "public, max-age=3600")
		_, _ = w.Write(icon)
	})
}
