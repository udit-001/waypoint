package web

import "embed"

//go:embed index.html css js manifest.json sw.js icons fonts
var Files embed.FS
