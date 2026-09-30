package docs

import (
	"bytes"
	"os"
	"regexp"
	"testing"

	"oss-max/internal/domain"
	"oss-max/internal/storage"
)

// Посмотреть результат глазами:
// BULLETIN_PDF=/tmp/bulletin.pdf go test -run TestBulletins ./internal/docs
func TestBulletins(t *testing.T) {
	meeting := storage.Meeting{
		Address:  "г. Примерск, ул. Тестовая, д. 1",
		Question: "Установить шлагбаум на въезде во двор и включить плату за его обслуживание в единый платёжный документ",
	}
	bulletins := []storage.Bulletin{
		{
			Kind: "person", Name: "Кузнецова Анна Николаевна", Choice: domain.ChoiceFor,
			Rights: []storage.Right{
				{FlatNumber: "1", RegistrationNumber: "00:00:0000000:1002-00/000/2005-2", RegistrationDate: "24.11.2005",
					FlatArea: 87.9, CadastralNumber: "00:00:0000000:101", OwnershipType: "Общая долевая собственность",
					ShareText: "1/2", Share: 0.5, PremisesType: "Квартира", OwnedArea: 43.95},
				{FlatNumber: "14", RegistrationNumber: "00:00:0000000:1040-00/000/2019-1", RegistrationDate: "03.04.2019",
					FlatArea: 41.2, CadastralNumber: "00:00:0000000:114", OwnershipType: "Собственность",
					Share: 1, PremisesType: "Квартира", OwnedArea: 41.2},
			},
		},
		{
			Kind: "org", Name: "ООО «Тестовое общество 1»",
			Rights: []storage.Right{
				{FlatNumber: "Н-1", RegistrationNumber: "ТЕСТ-00:00:0000000:0001/2019/1", RegistrationDate: "04.10.2019",
					FlatArea: 274.4, CadastralNumber: "00:00:0000000:0001", OwnershipType: "Собственность",
					ShareText: "1", Share: 1, PremisesType: "Нежилое помещение", OwnedArea: 274.4},
			},
		},
	}

	var out bytes.Buffer
	if err := Bulletins(&out, meeting, bulletins); err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(out.Bytes(), []byte("%PDF-")) {
		t.Fatal("на выходе не PDF")
	}
	// Три листа на каждого собственника.
	pages := regexp.MustCompile(`/Type /Page[^s]`).FindAll(out.Bytes(), -1)
	if len(pages) != 6 {
		t.Errorf("листов %d, хотели 6", len(pages))
	}

	if path := os.Getenv("BULLETIN_PDF"); path != "" {
		if err := os.WriteFile(path, out.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}
