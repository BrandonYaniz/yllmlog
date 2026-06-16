package migrations

import "embed"

// FS contains yllmlog SQLite migration files.
//
//go:embed *.sql
var FS embed.FS
