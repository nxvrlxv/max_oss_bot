package storage

import (
	"context"

	"github.com/jackc/pgx/v5"

	"oss-max/internal/domain"
)

// Votes — все голоса собрания, включая неподтверждённые:
// отсеивает их ядро, а не запрос.
func (s *Store) Votes(ctx context.Context, meetingID int) ([]domain.Vote, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT vt.owner_id, o.flat_id, vt.value, vt.status, vt.weight_area
		FROM votes vt JOIN owners o ON o.id = vt.owner_id
		WHERE vt.voting_id = $1
		ORDER BY vt.id`, meetingID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Vote, error) {
		var v domain.Vote
		err := row.Scan(&v.OwnerID, &v.UnitID, &v.Choice, &v.Status, &v.Area)
		return v, err
	})
}

// Result считает итог собрания на текущий момент.
func (s *Store) Result(ctx context.Context, meetingID int) (domain.Result, error) {
	meeting, err := s.Meeting(ctx, meetingID)
	if err != nil {
		return domain.Result{}, err
	}
	votes, err := s.Votes(ctx, meetingID)
	if err != nil {
		return domain.Result{}, err
	}
	return domain.Evaluate(meeting.TotalArea, votes, meeting.Rule), nil
}
