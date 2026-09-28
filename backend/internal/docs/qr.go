// Package docs — файлы для людей: QR-код приглашения, бюллетень,
// справка и протокол.
package docs

import (
	"fmt"

	"github.com/skip2/go-qrcode"
)

// QR рисует ссылку-приглашение PNG-картинкой 512×512: для дашборда
// инициатора, публикации в домовой чат и объявления в подъезде.
// Уровень коррекции Medium переживает до 15% повреждений — хватит
// для потёртой распечатки на двери. Белую рамку в 4 модуля библиотека
// рисует сама: без неё камеры плохо находят код.
func QR(link string) ([]byte, error) {
	png, err := qrcode.Encode(link, qrcode.Medium, 512)
	if err != nil {
		return nil, fmt.Errorf("QR-код: %w", err)
	}
	return png, nil
}
