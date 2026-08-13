package migrations

import "embed"

// FS contains the ordered SQL migrations for this service.
//
//go:embed *.sql
var FS embed.FS
