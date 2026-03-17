package pagechat

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed web/*
var webFS embed.FS

func webHandler() http.Handler {
	sub, _ := fs.Sub(webFS, "web")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Serve index.html for the root path directly to avoid redirect loops.
		if r.URL.Path == "/" {
			http.ServeFileFS(w, r, sub, "index.html")
			return
		}
		http.FileServerFS(sub).ServeHTTP(w, r)
	})
}
