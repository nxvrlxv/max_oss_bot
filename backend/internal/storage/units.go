package storage

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"oss-max/internal/domain"
	"oss-max/internal/registry"
)

// ImportRegistry заменяет реестр собрания целиком: старые помещения,
// собственники и голоса удаляются каскадом. Разрешено только в черновике —
// после публикации реестр — это снимок, по которому уже голосуют.
func (s *Store) ImportRegistry(ctx context.Context, meetingID int, flats []registry.Flat) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		var status string
		err := tx.QueryRow(ctx,
			`SELECT status FROM votings WHERE id = $1 FOR UPDATE`, meetingID).Scan(&status)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if status != MeetingDraft {
			return ErrNotDraft
		}

		if _, err := tx.Exec(ctx, `DELETE FROM flats WHERE voting_id = $1`, meetingID); err != nil {
			return err
		}

		// Сначала все помещения одним батчем, затем собственники —
		// им нужны id помещений.
		flatBatch := &pgx.Batch{}
		for _, flat := range flats {
			flatBatch.Queue(`
				INSERT INTO flats (voting_id, number, area, cadastral_number, premises_type)
				VALUES ($1, $2, $3, NULLIF($4, ''), NULLIF($5, ''))
				RETURNING id`,
				meetingID, flat.Number, flat.Area, flat.CadastralNumber, flat.PremisesType)
		}

		flatIDs := make([]int, len(flats))
		results := tx.SendBatch(ctx, flatBatch)
		for i, flat := range flats {
			if err := results.QueryRow().Scan(&flatIDs[i]); err != nil {
				results.Close()
				return fmt.Errorf("помещение %s: %w", flat.Number, err)
			}
		}
		if err := results.Close(); err != nil {
			return err
		}

		ownerBatch := &pgx.Batch{}
		for i, flat := range flats {
			for _, owner := range flat.Owners {
				fullName, orgName := owner.Name, ""
				if owner.Kind == "org" {
					fullName, orgName = "", owner.Name
				}
				ownerBatch.Queue(`
					INSERT INTO owners (flat_id, kind, full_name, org_name, share, owned_area, ownership_doc)
					VALUES ($1, $2, NULLIF($3, ''), NULLIF($4, ''), $5, $6, NULLIF($7, ''))`,
					flatIDs[i], owner.Kind, fullName, orgName, owner.Share, owner.OwnedArea, owner.OwnershipDoc)
			}
		}
		if err := tx.SendBatch(ctx, ownerBatch).Close(); err != nil {
			return fmt.Errorf("собственники: %w", err)
		}
		return nil
	})
}

// Registry возвращает помещения и собственников собрания для подсчёта.
func (s *Store) Registry(ctx context.Context, meetingID int) ([]domain.Unit, []domain.Owner, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, number, COALESCE(entrance, 0), area
		FROM flats WHERE voting_id = $1
		ORDER BY id`, meetingID)
	if err != nil {
		return nil, nil, err
	}
	units, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Unit, error) {
		var u domain.Unit
		err := row.Scan(&u.UnitID, &u.Number, &u.Entrance, &u.Area)
		return u, err
	})
	if err != nil {
		return nil, nil, err
	}

	rows, err = s.pool.Query(ctx, `
		-- Долю пересчитываем из площадей: в share всего шесть знаков.
		SELECT o.id, o.flat_id, COALESCE(o.owned_area / f.area, o.share)
		FROM owners o JOIN flats f ON f.id = o.flat_id
		WHERE f.voting_id = $1
		ORDER BY o.id`, meetingID)
	if err != nil {
		return nil, nil, err
	}
	owners, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Owner, error) {
		var o domain.Owner
		err := row.Scan(&o.OwnerID, &o.UnitID, &o.Share)
		return o, err
	})
	if err != nil {
		return nil, nil, err
	}

	return units, owners, nil
}

// Flat — помещение для выбора в мини-приложении: без ФИО собственников.
type Flat struct {
	ID     int     `json:"id"`
	Number string  `json:"number"`
	Area   float64 `json:"area"`
}

// Flats — помещения собрания в порядке реестра.
func (s *Store) Flats(ctx context.Context, meetingID int) ([]Flat, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, number, area FROM flats WHERE voting_id = $1 ORDER BY id`, meetingID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (Flat, error) {
		var f Flat
		err := row.Scan(&f.ID, &f.Number, &f.Area)
		return f, err
	})
}

// FlatGap — помещение, по которому ещё не учтены все голоса.
type FlatGap struct {
	Number string   `json:"number"`
	Area   float64  `json:"area"`   // сколько метров с него ещё можно собрать
	Owners []string `json:"owners"` // кто из реестра не проголосовал
}

// Coverage — охват собрания по помещениям: для дашборда инициатора.
type Coverage struct {
	FlatsTotal int       `json:"flats_total"`
	FlatsVoted int       `json:"flats_voted"` // хотя бы один учтённый голос
	NotVoted   []FlatGap `json:"not_voted"`   // крупные сверху — это и есть маршрут обхода
}

// Coverage считает, кого ещё не хватает. Неподтверждённые голоса
// не учитываются: по таким помещениям обходить всё равно придётся.
func (s *Store) Coverage(ctx context.Context, meetingID int) (Coverage, error) {
	var c Coverage
	err := s.pool.QueryRow(ctx, `
		SELECT
			(SELECT count(*) FROM flats WHERE voting_id = $1),
			(SELECT count(DISTINCT o.flat_id)
			 FROM votes v JOIN owners o ON o.id = v.owner_id
			 WHERE v.voting_id = $1 AND v.status = 'confirmed')`,
		meetingID).Scan(&c.FlatsTotal, &c.FlatsVoted)
	if err != nil {
		return c, err
	}

	rows, err := s.pool.Query(ctx, `
		SELECT f.number,
		       SUM(COALESCE(o.owned_area, ROUND(f.area * o.share, 2))) AS left_area,
		       array_agg(COALESCE(o.full_name, o.org_name, '') ORDER BY o.id)
		FROM owners o
		JOIN flats f ON f.id = o.flat_id
		LEFT JOIN votes v ON v.owner_id = o.id AND v.status = 'confirmed'
		WHERE f.voting_id = $1 AND v.id IS NULL
		GROUP BY f.id
		ORDER BY left_area DESC, f.id`, meetingID)
	if err != nil {
		return c, err
	}
	c.NotVoted, err = pgx.CollectRows(rows, func(row pgx.CollectableRow) (FlatGap, error) {
		var g FlatGap
		err := row.Scan(&g.Number, &g.Area, &g.Owners)
		return g, err
	})
	return c, err
}
