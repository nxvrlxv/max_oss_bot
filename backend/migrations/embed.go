// Package migrations вшивает SQL-файлы в бинарник: образу не нужна
// папка migrations рядом, а тесты накатывают ту же схему, что и прод.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
