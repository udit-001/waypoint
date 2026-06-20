package web

import "embed"

//go:embed index.html offline.html css js vendor manifest.json sw.js icons fonts
var Files embed.FS
