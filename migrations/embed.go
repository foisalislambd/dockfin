package migrations

import "embed"

// FS holds Goose SQL migrations embedded into the binary.
// Single source of truth: edit *.sql in this directory only.
//
//go:embed *.sql
var FS embed.FS
