package migrations

import "embed"

// New schema changes go in a new NNN_name.sql file. We never edit a migration
// once it's merged.
//
//go:embed *.sql
var FS embed.FS
