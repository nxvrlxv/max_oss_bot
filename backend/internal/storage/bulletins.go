package storage

import (
	"context"
	"sort"
	"strconv"
	"strings"

	"oss-max/internal/domain"
)

// Right — право собственника на помещение: строка таблицы в бюллетене.
// Реквизиты — как в выписке ЕГРН, пустые остаются пустыми.
type Right struct {
	FlatNumber         string
	RegistrationNumber string
	RegistrationDate   string
	FlatArea           float64
	CadastralNumber    string
	OwnershipType      string
	ShareText          string  // доля в записи выписки: «1/2»; пусто, если её там не было
	Share              float64 // доля числом — на случай пустого ShareText
	PremisesType       string
	OwnedArea          float64 // долевая площадь — по ней бланки для обхода идут крупными вперёд
}

// Bulletin — бюллетень одного собственника: он сам и все его доли в доме.
type Bulletin struct {
	Kind   string // person или org — от этого зависит вариант бланка
	Name   string
	Rights []Right
	Choice domain.Choice // ответ из приложения; пусто — собственник отметит от руки
}

// OwnBulletin — бюллетень того, кто голосует в приложении: собственник,
// подтверждённый за ним инициатором, с его ответом. Один аккаунт — один
// человек, поэтому бюллетень не больше одного. Пусто, пока ни одна заявка
// не подтверждена: без сверки с реестром бланк выдавать не на кого.
func (s *Store) OwnBulletin(ctx context.Context, meetingID int, maxID int64) ([]Bulletin, error) {
	return s.bulletins(ctx, `
		JOIN claims c ON c.owner_id = o.id AND c.voting_id = f.voting_id AND c.status = 'confirmed'
		JOIN users u ON u.id = c.user_id
		WHERE f.voting_id = $1 AND u.max_id = $2`, `COALESCE(c.choice, '')`,
		meetingID, maxID)
}

// BlankBulletins — пустые бюллетени для обхода: собственники, чей голос ещё
// не учтён. Неподтверждённые голоса не в счёт — как и в списке на дашборде:
// подписанный бланк всё равно понадобится.
func (s *Store) BlankBulletins(ctx context.Context, meetingID int) ([]Bulletin, error) {
	return s.bulletins(ctx, `
		LEFT JOIN votes v ON v.owner_id = o.id AND v.status = 'confirmed'
		WHERE f.voting_id = $1 AND v.id IS NULL`, `''`,
		meetingID)
}

// bulletins собирает строки реестра в бюллетени: доли одного человека
// в разных квартирах попадают в один бланк. Крупные собственники — первыми:
// это тот же маршрут обхода, что и список на дашборде.
func (s *Store) bulletins(ctx context.Context, where, choice string, args ...any) ([]Bulletin, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT o.id, o.kind, COALESCE(o.full_name, o.org_name, ''), `+choice+`,
		       f.number, COALESCE(o.ownership_doc, ''), COALESCE(o.registration_date, ''), f.area,
		       COALESCE(f.cadastral_number, ''), COALESCE(o.ownership_type, ''),
		       COALESCE(o.share_text, ''), o.share, COALESCE(f.premises_type, ''),
		       COALESCE(o.owned_area, ROUND(f.area * o.share, 2))
		FROM owners o
		JOIN flats f ON f.id = o.flat_id
		`+where+`
		ORDER BY f.id, o.id`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var (
		result []Bulletin
		owned  []float64
		index  = map[string]int{}
	)
	for rows.Next() {
		var (
			ownerID          int
			kind, name, vote string
			r                Right
		)
		err := rows.Scan(&ownerID, &kind, &name, &vote,
			&r.FlatNumber, &r.RegistrationNumber, &r.RegistrationDate, &r.FlatArea,
			&r.CadastralNumber, &r.OwnershipType, &r.ShareText, &r.Share, &r.PremisesType, &r.OwnedArea)
		if err != nil {
			return nil, err
		}

		// Без имени объединять не по чему — такой собственник получает свой бланк.
		key := kind + ":" + personKey(name)
		if strings.TrimSpace(name) == "" {
			key = "id:" + strconv.Itoa(ownerID)
		}
		i, seen := index[key]
		if !seen {
			i = len(result)
			index[key] = i
			result = append(result, Bulletin{Kind: kind, Name: name, Choice: domain.Choice(vote)})
			owned = append(owned, 0)
		}
		result[i].Rights = append(result[i].Rights, r)
		owned[i] += r.OwnedArea
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	order := make([]int, len(result))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(a, b int) bool { return owned[order[a]] > owned[order[b]] })
	sorted := make([]Bulletin, len(result))
	for i, j := range order {
		sorted[i] = result[j]
	}
	return sorted, nil
}
