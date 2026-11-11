package migrations

import "embed"

// FS содержит SQL-файлы миграций, вшитые в программу при сборке.
//
//go:embed *.sql
var FS embed.FS
