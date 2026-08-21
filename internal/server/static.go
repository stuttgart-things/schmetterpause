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
