package web

import "embed"

// Files is the presentation adapter (HTML/CSS/JS). It must not be imported
// by domain or application.
//
//go:embed index.html assets/*
var Files embed.FS
