package dashboard

import (
	"io/fs"

	"github.com/tazhate/chainplane/web"
)

// WebFS returns the embedded filesystem with the web UI assets.
func WebFS() fs.FS {
	return web.FS()
}
