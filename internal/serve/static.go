package serve

import (
	"embed"
	"io/fs"
)

//go:embed static/*
var staticFS embed.FS

// StaticFS returns the embedded filesystem rooted at static/.
func StaticFS() fs.FS {
	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		return staticFS
	}
	return sub
}
