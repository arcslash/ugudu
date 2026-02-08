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

	return http.FileServer(http.FS(sub))
}
