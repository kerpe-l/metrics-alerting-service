// Package migrations встраивает SQL-файлы миграций в бинарник через embed.FS.
package migrations

import "embed"

// FS — встроенная файловая система с SQL-файлами миграций (*.sql).
//
//go:embed *.sql
var FS embed.FS
