//go:build dev

package web

import (
	"io/fs"
	"os"
)

// staticFiles serves from disk in dev mode so edits are visible on refresh.
// Points at "internal/web" (not "internal/web/static") so that
// fs.Sub(staticFiles, "static") works the same as with embed.FS.
var staticFiles fs.FS = os.DirFS("internal/web")
