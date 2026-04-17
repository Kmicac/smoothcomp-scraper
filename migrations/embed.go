package migrations

import (
	"embed"
	"io/fs"
)

//go:embed postgres/*.sql sqlite/*.sql
var files embed.FS

func Files() fs.FS {
	return files
}
