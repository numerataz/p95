//go:build !dev

// Package web embeds the built web UI for the local viewer.
package web

import (
	"embed"
	"io/fs"
)

//go:embed dist/*
var distFS embed.FS

// DistFS returns the embedded web UI filesystem.
func DistFS() fs.FS {
	// Strip the "dist" prefix so files are served from root
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		return nil
	}
	return sub
}
