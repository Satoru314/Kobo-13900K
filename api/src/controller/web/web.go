// Package web serves the GUI built from app/ into dist.
package web

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed all:dist
var dist embed.FS

// Handler serves the built GUI files.
func Handler() http.Handler {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		panic(err) // dist is embedded at build time, so this cannot fail
	}
	return http.FileServerFS(sub)
}
