package registry

import (
	"strings"
	"testing"
)

func row(line int, flat, name, owned, area string) Ownership {
	return Ownership{
		SourceLine:   line,
		FlatNumber:   flat,
		FullName:     name,
		OwnedArea:    owned,
		FlatArea:     area,
		HouseAddress: "ул. Тестовая, д. 1",
	}
}

func TestGroup(t *testing.T) {
	flats, report, err := Group([]Ownership{
		row(2, "1", "Иванов Иван", "30", "60"),
		row(3, "2", "ООО «Ромашка»", "45.5", "45.5"),
		row(4, "1", "Петрова Анна", "30", "60"),
		row(5, "3", "Сидоров Олег", "20", "60"), // покрыта треть
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(flats) != 3 || len(flats[0].Owners) != 2 {
		t.Fatalf("помещения сгруппированы неверно: %+v", flats)
	}
	if flats[0].Owners[1].Name != "Петрова Анна" || flats[0].Owners[1].Share != "0.500000" {
		t.Errorf("второй собственник квартиры 1: %+v", flats[0].Owners[1])
	}
	if flats[1].Owners[0].Kind != "org" || flats[0].Owners[0].Kind != "person" {
		t.Errorf("вид собственника определён неверно: %+v", flats)
	}
	if flats[2].Owners[0].Share != "0.333333" {
		t.Errorf("доля 20 из 60 = %s, хотели 0.333333", flats[2].Owners[0].Share)
	}

	if report.Flats != 3 || report.Owners != 4 || report.FlatsArea != "165.50" || report.OwnedArea != "125.50" {
		t.Errorf("сводка: %+v", report)
	}
	if len(report.Warnings) != 1 || !strings.Contains(report.Warnings[0], "помещение 3") {
		t.Errorf("ждали одно предупреждение про неполное помещение 3: %v", report.Warnings)
	}
}

func TestGroupRejectsContradictions(t *testing.T) {
	cases := map[string][]Ownership{
		"разная площадь одного помещения": {
			row(2, "1", "Иванов", "30", "60"),
			row(3, "1", "Петров", "30", "65"),
		},
		"доли больше целого": {
			row(2, "1", "Иванов", "40", "60"),
			row(3, "1", "Петров", "30", "60"),
		},
		"пустой реестр": nil,
	}

	for name, rows := range cases {
		if flats, _, err := Group(rows); err == nil {
			t.Errorf("%s: принято %+v", name, flats)
		}
	}
}

func TestCheckTotalArea(t *testing.T) {
	report := Report{FlatsArea: "5900.00"}

	if warnings := report.CheckTotalArea("5900"); len(warnings) != 0 {
		t.Errorf("площади совпадают, а предупреждения есть: %v", warnings)
	}
	if warnings := report.CheckTotalArea("6000"); len(warnings) != 1 || !strings.Contains(warnings[0], "100,00") {
		t.Errorf("ждали нехватку 100 м²: %v", warnings)
	}
	if warnings := report.CheckTotalArea("5000"); len(warnings) != 1 {
		t.Errorf("ждали предупреждение о превышении: %v", warnings)
	}
}
