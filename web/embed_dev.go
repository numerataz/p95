//go:build dev

// Package web provides the web UI filesystem.
package web

import "io/fs"

// DistFS returns nil in dev mode - web UI served separately.
func DistFS() fs.FS {
	return nil
}
