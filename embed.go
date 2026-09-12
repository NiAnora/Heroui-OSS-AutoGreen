package main

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed all:web/dist
var webFS embed.FS

// webHandler 托管内嵌的前端静态资源。
func webHandler() http.Handler {
	dist, err := fs.Sub(webFS, "web/dist")
	if err != nil {
		panic(err)
	}
	return http.FileServer(http.FS(dist))
}
