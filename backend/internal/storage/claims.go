package storage

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"oss-max/internal/domain"
)

// Статусы заявки, как в CHECK таблицы claims.
const (
	ClaimPending   = "pending"
	ClaimConfirmed = "confirmed"
	ClaimRejected  = "rejected"
)

// Больше заявок одному человеку в собрании не нужно: у редкого
// собственника пять квартир в одном доме, а спам в очередь инициатора — частый случай.
const maxPendingClaims = 5

var (
	// ErrTooManyClaims — у человека уже слишком много неразобранных заявок.
	ErrTooManyClaims = errors.New("слишком много заявок без ответа")

	// ErrOwnerTaken — этот собственник уже привязан к другому человеку.
	ErrOwnerTaken = errors.New("собственник уже подтверждён за другим пользователем")
)

// Claim — заявка человека на помещение.
type Claim struct {
	ID         int           `json:"id"`
	FlatID     int           `json:"flat_id"`
	FlatNumber string        `json:"flat_number"`
	FlatArea   float64       `json:"flat_area"`
	Status     string        `json:"status"`
	Weight     float64       `json:"weight"` // вес голоса; 0, пока не подтверждена
	Choice     domain.Choice `json:"choice,omitempty"`
	VotedAt    *time.Time    `json:"voted_at,omitempty"`
	CreatedAt  time.Time     `json:"created_at"`
}

// RegistryOwner — собственник из реестра, к которому инициатор привязывает заявку.
type RegistryOwner struct {
	ID        int     `json:"id"`
	Name      string  `json:"name"`
	OwnedArea float64 `json:"owned_area"`
	Taken     bool    `json:"taken"` // уже подтверждён за кем-то
}

// PendingClaim — заявка в очереди инициатора: кто пришёл и кто есть в реестре.
type PendingClaim struct {
	Claim
	UserName string          `json:"user_name"` // имя в MAX
	Owners   []RegistryOwner `json:"owners"`
}

// ClaimFlat — человек говорит «это моя квартира». Повторная заявка
// на ту же квартиру возвращает существующую; отклонённую можно подать заново.
// fresh — заявка новая или поднята заново: о ней стоит сообщить инициатору.
func (s *Store) ClaimFlat(ctx context.Context, meetingID int, flatNumber string, user User) (claim Claim, fresh bool, err error) {
	var claimID int
	err = pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		if err := acceptingVotes(ctx, tx, meetingID); err != nil {
			return err
		}

		var flatID int
		err := tx.QueryRow(ctx,
			`SELECT id FROM flats WHERE voting_id = $1 AND number = $2`,
			meetingID, flatNumber).Scan(&flatID)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}

		userID, err := upsertUser(ctx, tx, user)
		if err != nil {
			return err
		}

		var pending int
		err = tx.QueryRow(ctx, `
			SELECT count(*) FROM claims
			WHERE voting_id = $1 AND user_id = $2 AND status = 'pending' AND flat_id <> $3`,
			meetingID, userID, flatID).Scan(&pending)
		if err != nil {
			return err
		}
		if pending >= maxPendingClaims {
			return ErrTooManyClaims
		}

		var previous string
		err = tx.QueryRow(ctx, `
			SELECT status FROM claims WHERE voting_id = $1 AND flat_id = $2 AND user_id = $3`,
			meetingID, flatID, userID).Scan(&previous)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		fresh = previous == "" || previous == ClaimRejected

		return tx.QueryRow(ctx, `
			INSERT INTO claims (voting_id, flat_id, user_id)
			VALUES ($1, $2, $3)
			ON CONFLICT (voting_id, flat_id, user_id) DO UPDATE
			SET status = CASE WHEN claims.status = 'rejected' THEN 'pending' ELSE claims.status END,
			    decided_at = CASE WHEN claims.status = 'rejected' THEN NULL ELSE claims.decided_at END
			RETURNING id`,
			meetingID, flatID, userID).Scan(&claimID)
	})
	if err != nil {
		return Claim{}, false, err
	}

	claims, err := s.claims(ctx, `c.id = $1`, claimID)
	if err != nil {
		return Claim{}, false, err
	}
	return claims[0], fresh, nil
}

// CancelClaim — «это не моя квартира»: человек сам отзывает неразобранную заявку.
func (s *Store) CancelClaim(ctx context.Context, meetingID, claimID int, maxID int64) error {
	tag, err := s.pool.Exec(ctx, `
		DELETE FROM claims c USING users u
		WHERE c.id = $2 AND c.voting_id = $1 AND c.user_id = u.id AND u.max_id = $3
		  AND c.status = 'pending'`,
		meetingID, claimID, maxID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// HasClaim — подавал ли человек заявку в это собрание, в любом статусе.
// Такому собрание видно и без приглашения: он уже в нём участвует.
func (s *Store) HasClaim(ctx context.Context, meetingID int, maxID int64) (bool, error) {
	var found bool
	err := s.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM claims c JOIN users u ON u.id = c.user_id
			WHERE c.voting_id = $1 AND u.max_id = $2
		)`, meetingID, maxID).Scan(&found)
	return found, err
}

// UserClaims — заявки человека в собрании, кроме отклонённых.
func (s *Store) UserClaims(ctx context.Context, meetingID int, maxID int64) ([]Claim, error) {
	return s.claims(ctx, `
		c.voting_id = $1 AND c.status <> 'rejected'
		AND c.user_id = (SELECT id FROM users WHERE max_id = $2)`,
		meetingID, maxID)
}

func (s *Store) claims(ctx context.Context, where string, args ...any) ([]Claim, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT c.id, f.id, f.number, f.area, c.status,
		       COALESCE(o.owned_area, 0), COALESCE(c.choice, ''), c.voted_at, c.created_at
		FROM claims c
		JOIN flats f ON f.id = c.flat_id
		LEFT JOIN owners o ON o.id = c.owner_id
		WHERE `+where+`
		ORDER BY f.id`, args...)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, scanClaim)
}

func scanClaim(row pgx.CollectableRow) (Claim, error) {
	var c Claim
	err := row.Scan(&c.ID, &c.FlatID, &c.FlatNumber, &c.FlatArea, &c.Status,
		&c.Weight, &c.Choice, &c.VotedAt, &c.CreatedAt)
	return c, err
}

// PendingClaims — очередь заявок инициатора, старые сверху. К каждой
// приложены собственники помещения из реестра: с ними инициатор и сверяет.
func (s *Store) PendingClaims(ctx context.Context, meetingID int) ([]PendingClaim, error) {
	rows, err := s.pool.Query(ctx, `
		-- Ответ собственника инициатору не показываем: подтверждение
		-- не должно зависеть от того, «за» человек или «против».
		SELECT c.id, f.id, f.number, f.area, c.status, 0::numeric, '',
		       NULL::timestamptz, c.created_at,
		       COALESCE(NULLIF(u.full_name, ''), u.username, 'id ' || u.max_id)
		FROM claims c
		JOIN flats f ON f.id = c.flat_id
		JOIN users u ON u.id = c.user_id
		WHERE c.voting_id = $1 AND c.status = 'pending'
		ORDER BY c.created_at, c.id`, meetingID)
	if err != nil {
		return nil, err
	}
	pending, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (PendingClaim, error) {
		var p PendingClaim
		err := row.Scan(&p.ID, &p.FlatID, &p.FlatNumber, &p.FlatArea, &p.Status, &p.Weight,
			&p.Choice, &p.VotedAt, &p.CreatedAt, &p.UserName)
		return p, err
	})
	if err != nil || len(pending) == 0 {
		return pending, err
	}

	rows, err = s.pool.Query(ctx, `
		SELECT o.flat_id, o.id, COALESCE(o.full_name, o.org_name, ''),
		       COALESCE(o.owned_area, ROUND(f.area * o.share, 2)),
		       EXISTS (SELECT 1 FROM claims c WHERE c.owner_id = o.id AND c.status = 'confirmed')
		FROM owners o
		JOIN flats f ON f.id = o.flat_id
		WHERE f.voting_id = $1
		  AND f.id IN (SELECT flat_id FROM claims WHERE voting_id = $1 AND status = 'pending')
		ORDER BY o.id`, meetingID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	owners := map[int][]RegistryOwner{}
	for rows.Next() {
		var flatID int
		var o RegistryOwner
		if err := rows.Scan(&flatID, &o.ID, &o.Name, &o.OwnedArea, &o.Taken); err != nil {
			return nil, err
		}
		owners[flatID] = append(owners[flatID], o)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for i := range pending {
		pending[i].Owners = owners[pending[i].FlatID]
	}
	return pending, nil
}

// PendingCount — сколько заявок ждёт инициатора.
func (s *Store) PendingCount(ctx context.Context, meetingID int) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx,
		`SELECT count(*) FROM claims WHERE voting_id = $1 AND status = 'pending'`, meetingID).Scan(&n)
	return n, err
}

// ConfirmClaim привязывает заявку к собственнику из реестра. Голос, отданный
// до подтверждения, переносится в подсчёт с весом этого собственника —
// даже если срок уже вышел: голос был подан вовремя.
func (s *Store) ConfirmClaim(ctx context.Context, meetingID, claimID, ownerID int) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		var (
			flatID, userID int
			choice         *string
			votedAt        *time.Time
		)
		err := tx.QueryRow(ctx, `
			SELECT flat_id, user_id, choice, voted_at FROM claims
			WHERE id = $1 AND voting_id = $2 AND status = 'pending'
			FOR UPDATE`, claimID, meetingID).Scan(&flatID, &userID, &choice, &votedAt)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}

		var taken bool
		err = tx.QueryRow(ctx, `
			SELECT EXISTS (SELECT 1 FROM claims WHERE owner_id = $2 AND status = 'confirmed')
			FROM owners WHERE id = $2 AND flat_id = $1
			FOR UPDATE`, flatID, ownerID).Scan(&taken)
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("собственник %d не из этого помещения: %w", ownerID, ErrNotFound)
		}
		if err != nil {
			return err
		}
		if taken {
			return ErrOwnerTaken
		}

		if _, err := tx.Exec(ctx, `
			UPDATE owners
			SET user_id = $2, status = 'confirmed', confirmed_at = now(),
			    claimed_at = (SELECT created_at FROM claims WHERE id = $3)
			WHERE id = $1`, ownerID, userID, claimID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			UPDATE claims SET status = 'confirmed', owner_id = $2, decided_at = now()
			WHERE id = $1`, claimID, ownerID); err != nil {
			return err
		}

		if choice == nil {
			return nil
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO votes (voting_id, owner_id, value, weight_area, status, created_at, updated_at)
			SELECT $1, o.id, $3, COALESCE(o.owned_area, ROUND(f.area * o.share, 2)), 'confirmed', $4, $4
			FROM owners o JOIN flats f ON f.id = o.flat_id
			WHERE o.id = $2
			ON CONFLICT (voting_id, owner_id) DO UPDATE
			SET value = EXCLUDED.value, updated_at = EXCLUDED.updated_at`,
			meetingID, ownerID, *choice, votedAt)
		return err
	})
}

// RejectClaim отклоняет заявку. Голос из неё в подсчёт не попадает.
func (s *Store) RejectClaim(ctx context.Context, meetingID, claimID int) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE claims SET status = 'rejected', decided_at = now()
		WHERE id = $1 AND voting_id = $2 AND status = 'pending'`, claimID, meetingID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// Vote записывает ответ человека по всем его квартирам в собрании.
// По подтверждённым заявкам голос сразу идёт в подсчёт, по остальным
// ждёт в заявке. Переголосование до срока меняет ответ, вес не трогает.
func (s *Store) Vote(ctx context.Context, meetingID int, maxID int64, choice domain.Choice) error {
	switch choice {
	case domain.ChoiceFor, domain.ChoiceAgainst, domain.ChoiceAbstain:
	default:
		return fmt.Errorf("неизвестный вариант ответа %q", choice)
	}

	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		if err := acceptingVotes(ctx, tx, meetingID); err != nil {
			return err
		}

		tag, err := tx.Exec(ctx, `
			UPDATE claims c SET choice = $3, voted_at = now()
			FROM users u
			WHERE c.voting_id = $1 AND c.user_id = u.id AND u.max_id = $2
			  AND c.status IN ('pending', 'confirmed')`,
			meetingID, maxID, string(choice))
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return ErrCannotVote
		}

		_, err = tx.Exec(ctx, `
			INSERT INTO votes (voting_id, owner_id, value, weight_area, status)
			SELECT c.voting_id, o.id, c.choice, COALESCE(o.owned_area, ROUND(f.area * o.share, 2)), 'confirmed'
			FROM claims c
			JOIN users u ON u.id = c.user_id
			JOIN owners o ON o.id = c.owner_id
			JOIN flats f ON f.id = o.flat_id
			WHERE c.voting_id = $1 AND u.max_id = $2 AND c.status = 'confirmed'
			ON CONFLICT (voting_id, owner_id) DO UPDATE
			SET value = EXCLUDED.value, updated_at = now()`,
			meetingID, maxID)
		return err
	})
}

// acceptingVotes проверяет, что собрание идёт и срок не вышел.
// Блокирует строку собрания до конца транзакции: публикация и
// завершение не проскочат между проверкой и записью.
func acceptingVotes(ctx context.Context, tx pgx.Tx, meetingID int) error {
	var open bool
	err := tx.QueryRow(ctx, `
		SELECT status = 'active' AND (ends_at IS NULL OR ends_at > now())
		FROM votings WHERE id = $1
		FOR SHARE`, meetingID).Scan(&open)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if !open {
		return ErrCannotVote
	}
	return nil
}

func upsertUser(ctx context.Context, tx pgx.Tx, user User) (int, error) {
	var id int
	err := tx.QueryRow(ctx, `
		INSERT INTO users (max_id, full_name, username)
		VALUES ($1, NULLIF($2, ''), NULLIF($3, ''))
		ON CONFLICT (max_id) DO UPDATE SET
			full_name = COALESCE(EXCLUDED.full_name, users.full_name),
			username  = COALESCE(EXCLUDED.username, users.username)
		RETURNING id`,
		user.MaxID, user.Name, user.Username).Scan(&id)
	return id, err
}
