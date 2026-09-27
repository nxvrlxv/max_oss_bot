package docs

import (
	"bytes"
	"image/png"
	"testing"
)

func TestQR(t *testing.T) {
	data, err := QR("https://max.ru/t675_hakaton_max_bot?startapp=join_0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatal(err)
	}

	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("не PNG: %v", err)
	}
	if size := img.Bounds().Size(); size.X != 512 || size.Y != 512 {
		t.Errorf("размер %v, хотели 512×512", size)
	}
	// Угол — это рамка, она должна быть белой.
	if r, g, b, _ := img.At(0, 0).RGBA(); r != 0xffff || g != 0xffff || b != 0xffff {
		t.Error("угол картинки не белый: рамка пропала")
	}

	if _, err := QR(""); err == nil {
		t.Error("пустая ссылка должна давать ошибку")
	}
}
