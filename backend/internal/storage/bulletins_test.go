package storage

import (
	"context"
	"testing"

	"oss-max/internal/domain"
	"oss-max/internal/registry"
)

// Бюллетень собирает доли одного человека из разных квартир, реквизиты
// права берёт из выписки, а ответ — из приложения после подтверждения.
func TestBulletins(t *testing.T) {
	db := openTest(t)
	ctx := context.Background()
	m := newMeeting(t, db, 10)

	right := func(name, area, share, number, date string) registry.Owner {
		return registry.Owner{Kind: "person", Name: name, OwnedArea: area, Share: "0.500000", ShareText: share,
			OwnershipType: "Общая долевая собственность", RegistrationNumber: number, RegistrationDate: date}
	}
	flats := []registry.Flat{
		{Number: "1", Area: "60", CadastralNumber: "00:00:0000000:101", PremisesType: "Квартира", Owners: []registry.Owner{
			right("Семёнов Иван", "30", "1/2", "00:00:0000000:101-1", "01.02.2003"),
			right("Петрова Анна", "30", "1/2", "00:00:0000000:101-2", "04.05.2006"),
		}},
		// Тот же человек, записанный иначе, — один бюллетень на обе квартиры.
		{Number: "2", Area: "40", PremisesType: "Квартира", Owners: []registry.Owner{
			{Kind: "person", Name: "семенов  иван", OwnedArea: "40", Share: "1.000000", OwnershipType: "Собственность"},
		}},
		{Number: "Н-1", Area: "100", PremisesType: "Нежилое помещение", Owners: []registry.Owner{
			{Kind: "org", Name: "ООО «Ромашка»", OwnedArea: "100", Share: "1.000000", ShareText: "1"},
		}},
	}
	if err := db.ImportRegistry(ctx, m.ID, flats); err != nil {
		t.Fatal(err)
	}
	if err := db.Publish(ctx, m.ID); err != nil {
		t.Fatal(err)
	}

	names := func(bulletins []Bulletin) []string {
		var out []string
		for _, b := range bulletins {
			out = append(out, b.Name)
		}
		return out
	}

	blanks, err := db.BlankBulletins(ctx, m.ID)
	if err != nil {
		t.Fatal(err)
	}
	// Крупные доли первыми: 100, 30 + 40, 30.
	if got := names(blanks); len(got) != 3 || got[0] != "ООО «Ромашка»" || got[1] != "Семёнов Иван" || got[2] != "Петрова Анна" {
		t.Fatalf("бланки для обхода: %v", got)
	}
	if blanks[0].Kind != "org" || len(blanks[1].Rights) != 2 {
		t.Errorf("юрлицо или доли одного человека собраны неверно: %+v", blanks[:2])
	}
	first := blanks[1].Rights[0]
	if first.RegistrationNumber != "00:00:0000000:101-1" || first.RegistrationDate != "01.02.2003" ||
		first.ShareText != "1/2" || first.OwnershipType != "Общая долевая собственность" || first.FlatArea != 60 {
		t.Errorf("реквизиты права: %+v", first)
	}

	// До подтверждения своего бюллетеня нет: сверять не с чем.
	owners, err := db.FlatOwners(ctx, m.ID, "1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.SubmitVote(ctx, m.ID, User{MaxID: 20}, domain.ChoiceAgainst, "1", owners[0].ID); err != nil {
		t.Fatal(err)
	}
	if own, err := db.OwnBulletin(ctx, m.ID, 20); err != nil || len(own) != 0 {
		t.Fatalf("бюллетень до подтверждения: %+v %v", own, err)
	}

	pending, err := db.PendingClaims(ctx, m.ID)
	if err != nil || len(pending) != 1 {
		t.Fatalf("заявки: %+v %v", pending, err)
	}
	if err := db.ConfirmRequestedClaim(ctx, m.ID, pending[0].ID); err != nil {
		t.Fatal(err)
	}

	own, err := db.OwnBulletin(ctx, m.ID, 20)
	if err != nil || len(own) != 1 {
		t.Fatalf("свой бюллетень: %+v %v", own, err)
	}
	if own[0].Name != "Семёнов Иван" || own[0].Choice != domain.ChoiceAgainst ||
		len(own[0].Rights) != 1 || own[0].Rights[0].FlatNumber != "1" {
		t.Errorf("свой бюллетень — подтверждённая доля с ответом: %+v", own[0])
	}

	// Учтённая доля уходит из обхода, неучтённая остаётся.
	blanks, err = db.BlankBulletins(ctx, m.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got := names(blanks); len(got) != 3 || got[1] != "семенов  иван" || len(blanks[1].Rights) != 1 {
		t.Errorf("бланки после голоса: %v", got)
	}
}
