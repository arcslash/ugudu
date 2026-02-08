package api

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed ui/*
var uiFS embed.FS

// UIHandler returns an http.Handler that serves the embedded UI
func UIHandler() http.Handler {
	// Strip the "ui" prefix from the embedded filesystem
	sub, err := fs.Sub(uiFS, "ui")
	if err != nil {
		// Fallback to empty handler if embedding fails
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte("UI not available"))
		})
	}

	fileServer := http.FileServer(http.FS(sub))

	// Wrap with cache control headers
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// No cache for HTML, cache assets with hashes
		if r.URL.Path == "/" || r.URL.Path == "/index.html" {
			w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
			w.Header().Set("Pragma", "no-cache")
			w.Header().Set("Expires", "0")
		}
		fileServer.ServeHTTP(w, r)
	})
}
