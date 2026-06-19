package web

import "embed"

//go:embed index.html css js manifest.json sw.js icons
var Files embed.FS
