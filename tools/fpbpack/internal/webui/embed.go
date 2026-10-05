package webui

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed all:dist
var embedded embed.FS

// Handler serves the static GUI embedded in the release binary.
func Handler() http.Handler {
	root, err := fs.Sub(embedded, "dist")
	if err != nil {
		return unavailableHandler(err.Error())
	}
	return StaticHandler(root)
}

// StaticHandler serves a Next.js static export. It is separate so routing can
// be unit-tested without requiring a frontend build.
func StaticHandler(root fs.FS) http.Handler {
	if _, err := fs.Stat(root, "index.html"); err != nil {
		return unavailableHandler("GUI assets are not built into this binary")
	}
	return http.FileServer(http.FS(root))
}

func unavailableHandler(message string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, message, http.StatusServiceUnavailable)
	})
}
