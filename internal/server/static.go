package server

import (
	"io/fs"
	"net/http"

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
		// The assets are embedded in the binary and change only with a new
		// image. An hour is a safe compromise as long as the filenames carry
		// no content hash.
		w.Header().Set("Cache-Control", "public, max-age=3600")
		fileServer.ServeHTTP(w, r)
	}))
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
