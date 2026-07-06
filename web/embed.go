// Package web embeds the Yoshi Pi-hole dashboard's static assets directly
// into the binary, so no separate web server or Node build step is needed
// at runtime.
package web

import (
	"embed"
	"io/fs"
)

//go:embed dist
var distFS embed.FS

// FS returns the dashboard's static assets rooted at dist/, ready to be
// served with http.FileServer(http.FS(web.FS())).
func FS() fs.FS {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		panic(err) // dist/ is embedded at compile time; this can't fail at runtime
	}
	return sub
}
