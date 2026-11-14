package web

import "embed"

//go:embed index.html css js shaders
var Files embed.FS
