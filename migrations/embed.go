// Package migrations embeds the SQL migration files into the binary,
// so the service can migrate the database on startup without extra files.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
