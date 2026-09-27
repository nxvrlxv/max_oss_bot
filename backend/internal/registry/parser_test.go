package registry

import (
	"encoding/csv"
	"strings"
	"testing"
)

func sample(sep rune, owned string) string {
	var b strings.Builder
	w := csv.NewWriter(&b)
	w.Comma = sep
	_ = w.Write([]string{"№ помещения", "Адрес объекта", "Правообладатель (правообладатели)", "Долевая площадь, м²", "Общая площадь, м²"})
	_ = w.Write([]string{"12А", "г. Примерск, ул. Тестовая, д. 1А, кв. 12А", "  Иванов   Иван Иванович ", owned, "60.50"})
	w.Flush()
	return "\uFEFF" + b.String()
}

func TestParseCSV(t *testing.T) {
	for _, sep := range []rune{';', ',', '\t'} {
		rows, err := ParseCSV(strings.NewReader(sample(sep, "30,25")))
		if err != nil {
			t.Fatal(err)
		}
		got := rows[0]
		if len(rows) != 1 || got.FullName != "Иванов Иван Иванович" || got.HouseAddress != "г. Примерск, ул. Тестовая, д. 1А" || got.FlatNumber != "12А" || got.OwnedArea != "30.25" || got.FlatArea != "60.5" {
			t.Fatalf("unexpected: %+v", rows)
		}
	}
}

func TestInvalidAreaRejectsEntireImport(t *testing.T) {
	for _, value := range []string{"", "0", "-1", "NaN", "1e2", "61", "60.500000000000000001"} {
		input := sample(';', "30") + strings.SplitN(sample(';', value), "\n", 2)[1]
		rows, err := ParseCSV(strings.NewReader(input))
		if err == nil || rows != nil {
			t.Fatalf("accepted %q: %+v, %v", value, rows, err)
		}
	}
}

func TestMalformedCSV(t *testing.T) {
	for _, input := range []string{"", "name;area\nAlice;10", sample(';', "30") + "bad;row\n", string([]byte{0xff})} {
		if rows, err := ParseCSV(strings.NewReader(input)); err == nil || rows != nil {
			t.Fatalf("accepted malformed input: %q", input)
		}
	}
}

func TestSharedPremisesAndLegalEntity(t *testing.T) {
	input := sample(';', "30.25")
	input += strings.ReplaceAll(strings.SplitN(sample(';', "30.25"), "\n", 2)[1], "Иванов   Иван Иванович", "ООО «Тест»")
	rows, err := ParseCSV(strings.NewReader(input))
	if err != nil || len(rows) != 2 || rows[1].FullName != "ООО «Тест»" {
		t.Fatalf("%+v %v", rows, err)
	}
}

func extendedCSV(rows ...[]string) string {
	var b strings.Builder
	w := csv.NewWriter(&b)
	w.Comma = ';'
	_ = w.Write([]string{"№ помещения", "Адрес объекта", "Правообладатель (правообладатели)", "Долевая площадь, м²", "Общая площадь, м²", "Доля собственника", "Вид помещения", "Вид собственности", "Запись в ЕГРН, №", "Дата регистрации права", "Дата присвоения кадастрового номера", "Номер выписки", "Дата выписки", "Примечание"})
	for _, row := range rows {
		_ = w.Write(row)
	}
	w.Flush()
	return b.String()
}

func TestContinuationAndSourceFields(t *testing.T) {
	first := []string{"5", "Дом 1, кв. 5", "Иванов Иван", "30", "60", " 1/2 ", "Квартира", "Долевая собственность", "право-1", "01.02.2020", "01.01.2000", "выписка-1", "20.09.2026", " исходный текст "}
	next := []string{"", "", "Петров Пётр", "30", "", "1/2", "", "", "", "", "", "", "", ""}
	rows, err := ParseCSV(strings.NewReader(extendedCSV(first, next)))
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("unexpected rows: %+v", rows)
	}
	a, b := rows[0], rows[1]
	if a.Share != " 1/2 " || a.RegistrationNumber != "право-1" || a.RegistrationDate != "01.02.2020" || a.ExtractNumber != "выписка-1" || a.ExtractDate != "20.09.2026" || a.SourceFields["Примечание"] != " исходный текст " {
		t.Fatalf("lost source data: %+v", a)
	}
	if b.FlatNumber != "5" || b.FlatArea != "60" || b.OwnedArea != "30" || b.ObjectAddress != "Дом 1, кв. 5" || b.HouseAddress != "Дом 1" || b.PremisesType != "Квартира" || b.CadastralAssignmentDate != "01.01.2000" || b.SourceLine != 3 {
		t.Fatalf("lost premises: %+v", b)
	}
	if b.OwnershipType != "" || b.RegistrationNumber != "" || b.RegistrationDate != "" || b.ExtractNumber != "" || b.SourceFields["№ помещения"] != "" {
		t.Fatalf("inherited owner/source fields: %+v", b)
	}
}

func TestAmbiguousContinuationsRejectEntireImport(t *testing.T) {
	first := []string{"5", "Дом 1, кв. 5", "Иванов Иван", "30", "60", "1/2", "", "", "", "", "", "", "", ""}
	for _, tc := range []struct {
		index int
		value string
	}{{1, "Дом 2, кв. 5"}, {4, "70"}, {2, "Иванов Иван\nПетров Пётр"}, {2, ""}, {3, ""}} {
		next := append([]string(nil), first...)
		next[0] = ""
		next[tc.index] = tc.value
		rows, err := ParseCSV(strings.NewReader(extendedCSV(first, next)))
		if err == nil || rows != nil {
			t.Fatalf("accepted ambiguous row: %+v", next)
		}
	}
	first[0] = ""
	if rows, err := ParseCSV(strings.NewReader(extendedCSV(first))); err == nil || rows != nil {
		t.Fatal("accepted orphan continuation")
	}
}
