package registry

import (
	"fmt"
	"math/big"
	"regexp"
	"strings"
)

// Flat — помещение с собственниками в виде, готовом к записи в базу.
// Площади — десятичные строки: в Postgres они уходят в NUMERIC без float.
type Flat struct {
	Number          string
	Area            string
	CadastralNumber string
	PremisesType    string
	Owners          []Owner
}

// Owner — собственник помещения из реестра.
type Owner struct {
	Kind         string // person или org, как в CHECK таблицы owners
	Name         string // ФИО или название организации
	OwnedArea    string // долевая площадь, м² — вес голоса
	Share        string // OwnedArea / Area, шесть знаков — для бланка
	OwnershipDoc string // запись ЕГРН, если есть
	SourceLine   int
}

// Report — сводка для инициатора: что загрузилось и что выглядит подозрительно.
type Report struct {
	HouseAddress string
	Flats        int
	Owners       int
	FlatsArea    string   // сумма площадей помещений, м²
	OwnedArea    string   // сумма долевых площадей, м²
	Warnings     []string // не блокируют импорт, но их стоит показать
}

// Организации в выписках пишутся по-разному; ловим самые частые формы.
// ИП — физическое лицо, в список не входит.
var orgPattern = regexp.MustCompile(`(?i)^(ООО|АО|ПАО|ЗАО|ОАО|НАО|ГУП|МУП|ФГУП|ТСЖ|ТСН|ЖСК)[\s«"]|` +
	`общество с ограниченной|акционерное общество|российская федерация|` +
	`муниципальн|городской округ|администрация|департамент`)

// Group собирает строки реестра в помещения. Ошибка — если одно помещение
// описано противоречиво или доли в нём больше целого: такой реестр
// дал бы неверные веса голосов, и импорт отклоняется целиком.
func Group(rows []Ownership) ([]Flat, Report, error) {
	var report Report
	if len(rows) == 0 {
		return nil, report, fmt.Errorf("в реестре нет ни одной строки")
	}

	index := map[string]int{}
	var flats []Flat
	owned := map[string]*big.Rat{}
	addresses := map[string]int{}

	for _, row := range rows {
		addresses[row.HouseAddress]++

		i, seen := index[row.FlatNumber]
		if !seen {
			i = len(flats)
			index[row.FlatNumber] = i
			flats = append(flats, Flat{
				Number:          row.FlatNumber,
				Area:            row.FlatArea,
				CadastralNumber: row.CadastralNumber,
				PremisesType:    row.PremisesType,
			})
			owned[row.FlatNumber] = new(big.Rat)
		}

		flat := &flats[i]
		if compareDecimals(flat.Area, row.FlatArea) != 0 {
			return nil, report, fmt.Errorf("строка %d: у помещения %s площадь %s, выше было %s",
				row.SourceLine, row.FlatNumber, ru(row.FlatArea), ru(flat.Area))
		}

		flat.Owners = append(flat.Owners, Owner{
			Kind:         ownerKind(row.FullName),
			Name:         row.FullName,
			OwnedArea:    row.OwnedArea,
			Share:        share(row.OwnedArea, row.FlatArea),
			OwnershipDoc: ownershipDoc(row),
			SourceLine:   row.SourceLine,
		})
		owned[row.FlatNumber].Add(owned[row.FlatNumber], rat(row.OwnedArea))
	}

	flatsArea, ownedArea := new(big.Rat), new(big.Rat)
	for _, flat := range flats {
		area := rat(flat.Area)
		sum := owned[flat.Number]

		switch sum.Cmp(area) {
		case 1:
			return nil, report, fmt.Errorf("помещение %s: сумма долей %s м² больше площади %s м²",
				flat.Number, ru(sum.FloatString(2)), ru(flat.Area))
		case -1:
			report.Warnings = append(report.Warnings, fmt.Sprintf(
				"помещение %s: собственники покрывают %s м² из %s м² — часть собственников не указана",
				flat.Number, ru(sum.FloatString(2)), ru(flat.Area)))
		}

		flatsArea.Add(flatsArea, area)
		ownedArea.Add(ownedArea, sum)
		report.Owners += len(flat.Owners)
	}

	report.HouseAddress = mostCommon(addresses)
	if len(addresses) > 1 {
		report.Warnings = append(report.Warnings, fmt.Sprintf(
			"в реестре %d разных адресов дома, основной — %s", len(addresses), report.HouseAddress))
	}
	report.Flats = len(flats)
	report.FlatsArea = flatsArea.FloatString(2)
	report.OwnedArea = ownedArea.FloatString(2)

	return flats, report, nil
}

// CheckTotalArea сверяет сумму площадей из реестра с общей площадью,
// которую ввёл инициатор. Расхождение — не ошибка, но без предупреждения
// неполный реестр молча занижает порог.
func (r Report) CheckTotalArea(totalArea string) []string {
	total, ok := new(big.Rat).SetString(totalArea)
	if !ok || total.Sign() <= 0 {
		return []string{"общая площадь дома не указана"}
	}

	flats := rat(r.FlatsArea)
	switch flats.Cmp(total) {
	case 1:
		return []string{fmt.Sprintf("помещения в реестре занимают %s м² — больше общей площади дома %s м²",
			ru(r.FlatsArea), ru(total.FloatString(2)))}
	case -1:
		missing := new(big.Rat).Sub(total, flats)
		return []string{fmt.Sprintf("в реестре не хватает %s м² до общей площади дома %s м²",
			ru(missing.FloatString(2)), ru(total.FloatString(2)))}
	}
	return nil
}

// ru — десятичная запятая в сообщениях для пользователя.
func ru(decimal string) string {
	return strings.Replace(decimal, ".", ",", 1)
}

func ownerKind(name string) string {
	if orgPattern.MatchString(strings.TrimSpace(name)) {
		return "org"
	}
	return "person"
}

func ownershipDoc(row Ownership) string {
	switch {
	case row.RegistrationNumber != "" && row.RegistrationDate != "":
		return fmt.Sprintf("%s от %s", row.RegistrationNumber, row.RegistrationDate)
	default:
		return row.RegistrationNumber
	}
}

// share — доля в праве с шестью знаками, как в owners.share.
// Округление вверх до минимальной доли: CHECK требует share > 0.
func share(ownedArea, flatArea string) string {
	value := new(big.Rat).Quo(rat(ownedArea), rat(flatArea))
	if value.Cmp(big.NewRat(1, 1_000_000)) < 0 {
		return "0.000001"
	}
	return value.FloatString(6)
}

// rat разбирает десятичную строку, уже проверенную парсером.
func rat(s string) *big.Rat {
	value, ok := new(big.Rat).SetString(s)
	if !ok {
		return new(big.Rat)
	}
	return value
}

func mostCommon(counts map[string]int) string {
	var best string
	for value, n := range counts {
		if n > counts[best] || (n == counts[best] && value < best) {
			best = value
		}
	}
	return best
}
