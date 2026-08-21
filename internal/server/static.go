package server

import (
	"io/fs"
	"net/http"

	"github.com/stuttgart-things/schmetterpause/web"
)

// staticHandler liefert die eingebetteten Assets unter /static/.
func staticHandler() http.Handler {
	assets, err := fs.Sub(web.Static, "static")
	if err != nil {
		// Kann nur brechen, wenn das embed-Verzeichnis fehlt — dann ist das
		// Binary kaputt gebaut und soll das sofort zeigen.
		panic("eingebettete assets nicht lesbar: " + err.Error())
	}

	fileServer := http.FileServerFS(assets)

	return http.StripPrefix("/static/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Die Assets sind ins Binary eingebettet und aendern sich nur mit
		// einem neuen Image. Eine Stunde ist ein sicherer Kompromiss, solange
		// die Dateinamen keinen Content-Hash tragen.
		w.Header().Set("Cache-Control", "public, max-age=3600")
		fileServer.ServeHTTP(w, r)
	}))
}
