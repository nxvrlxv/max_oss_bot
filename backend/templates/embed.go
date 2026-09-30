// Package templates вшивает в бинарник файлы для документов: базовые
// шрифты PDF кириллицу не содержат, а образу не нужна папка рядом.
package templates

import "embed"

// Fonts — Liberation Serif (SIL OFL 1.1, лицензия рядом): по метрикам
// совпадает с Times New Roman, которым набирают бумажные бланки ОСС.
//
//go:embed fonts/*.ttf
var Fonts embed.FS
