// Package web provides the embedded Fleet Status web UI.
package web

import (
	"embed"
	"io/fs"
)

//go:embed index.html
var Files embed.FS

// FS returns the embedded filesystem with web assets.
func FS() fs.FS {
	return Files
}
