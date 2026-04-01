package web

import (
	"embed"
	"io/fs"
	"log"
)

//go:embed all:templates all:static
var content embed.FS

// FS returns the template and static filesystems.
// The template FS has paths like "templates/layout.html".
// The static FS has paths like "css/style.css", "js/app.js".
func FS() (templateFS fs.FS, staticFS fs.FS) {
	staticFS, err := fs.Sub(content, "static")
	if err != nil {
		log.Fatal(err)
	}
	return content, staticFS
}
