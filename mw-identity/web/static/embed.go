package static

import "embed"

// Files contains the complete dependency-free Identity portal.
//
//go:embed *.html *.css *.js *.svg
var Files embed.FS
