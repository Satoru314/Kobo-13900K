// Package migrations embeds the SQL files so the kobo binary can apply them at startup.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
