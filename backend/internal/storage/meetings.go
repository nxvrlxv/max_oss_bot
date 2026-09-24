package storage

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"oss-max/internal/domain"
)

// Статусы собрания, как в CHECK таблицы votings.
const (
	MeetingDraft    = "draft"
	MeetingActive   = "active"
	MeetingFinished = "finished"
)

// Meeting — собрание вместе с домом, к которому оно относится.
type Meeting struct {
	ID             int
	InitiatorID    int64 // max_id создавшего
	ChatID         int64 // домовой чат; 0, если не привязан
	HouseID        int
	Address        string
	Question       string
	Rule           domain.Rule
	TotalArea      float64
	EntrancesCount int
	InviteToken    string
	Status         string
	StartsAt       *time.Time
	EndsAt         *time.Time
}

// NewMeeting — то, что инициатор вводит в мини-приложении.
type NewMeeting struct {
	InitiatorMaxID int64
	Address        string
	Question       string
	Rule           domain.Rule
	TotalArea      float64 // вводится вручную, не суммой из реестра
	EntrancesCount int
	StartsAt       *time.Time
	EndsAt         *time.Time
}

// ErrInvalid — данные собрания не проходят проверку; текст ошибки можно показать пользователю.
type ErrInvalid struct{ Reason string }

func (e ErrInvalid) Error() string { return e.Reason }

func (m *NewMeeting) validate() error {
	switch {
	case strings.TrimSpace(m.Address) == "":
		return ErrInvalid{"Укажите адрес дома"}
	case strings.TrimSpace(m.Question) == "":
		return ErrInvalid{"Сформулируйте вопрос"}
	case len([]rune(m.Question)) > 1000:
		return ErrInvalid{"Вопрос длиннее 1000 символов"}
	case m.TotalArea <= 0:
		return ErrInvalid{"Общая площадь дома должна быть больше нуля"}
	case !m.Rule.Valid():
		return ErrInvalid{"Некорректное правило принятия решения"}
	case m.EndsAt != nil && m.StartsAt != nil && !m.EndsAt.After(*m.StartsAt):
		return ErrInvalid{"Окончание голосования должно быть позже начала"}
	}
	if m.EntrancesCount < 1 {
		m.EntrancesCount = 1
	}
	return nil
}

const meetingSelect = `
	SELECT v.id, u.max_id, COALESCE(h.chat_id, 0), h.id, h.address,
	       v.question, v.rule_json, v.total_area, v.entrances_count,
	       v.invite_token, v.status, v.starts_at, v.ends_at
	FROM votings v
	JOIN houses h ON h.id = v.house_id
	JOIN users u ON u.id = v.initiator_user_id`

func scanMeeting(row pgx.Row) (Meeting, error) {
	var m Meeting
	err := row.Scan(&m.ID, &m.InitiatorID, &m.ChatID, &m.HouseID, &m.Address,
		&m.Question, &m.Rule, &m.TotalArea, &m.EntrancesCount,
		&m.InviteToken, &m.Status, &m.StartsAt, &m.EndsAt)
	return m, err
}

// CreateMeeting создаёт черновик собрания. Дом заводится без чата:
// чат привязывается позже, когда бота добавят в домовой чат.
func (s *Store) CreateMeeting(ctx context.Context, m NewMeeting) (Meeting, error) {
	if err := m.validate(); err != nil {
		return Meeting{}, err
	}

	token, err := inviteToken()
	if err != nil {
		return Meeting{}, err
	}

	var id int
	err = pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		// Инициатор мог прийти сразу в мини-приложение, минуя бота.
		var userID int
		err := tx.QueryRow(ctx, `
			INSERT INTO users (max_id) VALUES ($1)
			ON CONFLICT (max_id) DO UPDATE SET max_id = EXCLUDED.max_id
			RETURNING id`, m.InitiatorMaxID).Scan(&userID)
		if err != nil {
			return err
		}

		var houseID int
		err = tx.QueryRow(ctx,
			`INSERT INTO houses (address) VALUES ($1) RETURNING id`,
			strings.TrimSpace(m.Address)).Scan(&houseID)
		if err != nil {
			return err
		}

		return tx.QueryRow(ctx, `
			INSERT INTO votings (house_id, initiator_user_id, question, rule_json,
			                     total_area, entrances_count, invite_token, starts_at, ends_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			RETURNING id`,
			houseID, userID, strings.TrimSpace(m.Question), m.Rule,
			m.TotalArea, m.EntrancesCount, token, m.StartsAt, m.EndsAt).Scan(&id)
	})
	if err != nil {
		return Meeting{}, fmt.Errorf("создание собрания: %w", err)
	}

	return s.Meeting(ctx, id)
}

// Meeting возвращает собрание по номеру или ErrNotFound.
func (s *Store) Meeting(ctx context.Context, meetingID int) (Meeting, error) {
	m, err := scanMeeting(s.pool.QueryRow(ctx, meetingSelect+` WHERE v.id = $1`, meetingID))
	if errors.Is(err, pgx.ErrNoRows) {
		return Meeting{}, ErrNotFound
	}
	return m, err
}

// MeetingByToken — собрание по токену приглашения из ссылки или QR, иначе ErrNotFound.
func (s *Store) MeetingByToken(ctx context.Context, token string) (Meeting, error) {
	m, err := scanMeeting(s.pool.QueryRow(ctx, meetingSelect+` WHERE v.invite_token = $1`, token))
	if errors.Is(err, pgx.ErrNoRows) {
		return Meeting{}, ErrNotFound
	}
	return m, err
}

// MeetingsByInitiator — последние собрания пользователя, новые сверху.
// Лимит — под клавиатуру бота: больше десятка кнопок не читается.
func (s *Store) MeetingsByInitiator(ctx context.Context, maxID int64) ([]Meeting, error) {
	return s.meetings(ctx, meetingSelect+`
		WHERE u.max_id = $1
		ORDER BY v.id DESC
		LIMIT 10`, maxID)
}

// MeetingsForUser — собрания, где человек инициатор или подал заявку:
// главный экран мини-приложения.
func (s *Store) MeetingsForUser(ctx context.Context, maxID int64) ([]Meeting, error) {
	return s.meetings(ctx, meetingSelect+`
		WHERE u.max_id = $1
		   OR v.id IN (SELECT c.voting_id FROM claims c JOIN users cu ON cu.id = c.user_id
		               WHERE cu.max_id = $1 AND c.status <> 'rejected')
		ORDER BY v.id DESC
		LIMIT 50`, maxID)
}

// MeetingsByChat — незавершённые собрания дома, привязанного к чату.
func (s *Store) MeetingsByChat(ctx context.Context, chatID int64) ([]Meeting, error) {
	return s.meetings(ctx, meetingSelect+`
		WHERE h.chat_id = $1 AND v.status <> 'finished'
		ORDER BY v.id DESC
		LIMIT 10`, chatID)
}

func (s *Store) meetings(ctx context.Context, query string, args ...any) ([]Meeting, error) {
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var found []Meeting
	for rows.Next() {
		m, err := scanMeeting(rows)
		if err != nil {
			return nil, err
		}
		found = append(found, m)
	}
	return found, rows.Err()
}

// BindChat привязывает домовой чат к собранию. Привязать может только
// инициатор: иначе поддельный callback отправит чужое собрание в чужой чат.
//
// Ключ дома — chat_id. Если чат уже привязан к другому дому, собрание
// переезжает в него, а опустевший дом удаляется: второе собрание
// в том же чате — это тот же дом.
func (s *Store) BindChat(ctx context.Context, meetingID int, chatID, initiatorMaxID int64) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		var houseID int
		err := tx.QueryRow(ctx, `
			SELECT v.house_id
			FROM votings v
			JOIN users u ON u.id = v.initiator_user_id
			WHERE v.id = $1 AND u.max_id = $2
			FOR UPDATE OF v`, meetingID, initiatorMaxID).Scan(&houseID)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}

		var chatHouseID int
		err = tx.QueryRow(ctx, `SELECT id FROM houses WHERE chat_id = $1`, chatID).Scan(&chatHouseID)
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			_, err = tx.Exec(ctx, `UPDATE houses SET chat_id = $1 WHERE id = $2`, chatID, houseID)
			return err
		case err != nil:
			return err
		case chatHouseID == houseID:
			return nil // повторное нажатие той же кнопки
		}

		if _, err := tx.Exec(ctx,
			`UPDATE votings SET house_id = $1 WHERE id = $2`, chatHouseID, meetingID); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `
			DELETE FROM houses h
			WHERE h.id = $1 AND NOT EXISTS (SELECT 1 FROM votings WHERE house_id = h.id)`,
			houseID)
		return err
	})
}

// UnbindChat — бота удалили из чата: публиковать туда больше некуда.
func (s *Store) UnbindChat(ctx context.Context, chatID int64) error {
	_, err := s.pool.Exec(ctx, `UPDATE houses SET chat_id = NULL WHERE chat_id = $1`, chatID)
	return err
}

// UpdateDraft меняет данные собрания, пока оно не опубликовано.
func (s *Store) UpdateDraft(ctx context.Context, meetingID int, m NewMeeting) error {
	if err := m.validate(); err != nil {
		return err
	}
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		var houseID int
		err := tx.QueryRow(ctx, `
			UPDATE votings
			SET question = $2, rule_json = $3, total_area = $4, entrances_count = $5,
			    starts_at = $6, ends_at = $7
			WHERE id = $1 AND status = 'draft'
			RETURNING house_id`,
			meetingID, strings.TrimSpace(m.Question), m.Rule, m.TotalArea, m.EntrancesCount,
			m.StartsAt, m.EndsAt).Scan(&houseID)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotDraft
		}
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `UPDATE houses SET address = $2 WHERE id = $1`,
			houseID, strings.TrimSpace(m.Address))
		return err
	})
}

// Closed — голосование окончено: завершено явно или вышел срок.
// Статус в базе при истечении срока не меняется сам — проверяем по времени.
func (m Meeting) Closed(now time.Time) bool {
	if m.Status == MeetingFinished {
		return true
	}
	return m.Status == MeetingActive && m.EndsAt != nil && !now.Before(*m.EndsAt)
}

// Publish переводит черновик в голосование. Возвращает ErrNotDraft,
// если собрание уже опубликовано, — повторная публикация не шлёт второе уведомление.
func (s *Store) Publish(ctx context.Context, meetingID int) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE votings
		SET status = 'active', starts_at = COALESCE(starts_at, now()), notice_sent_at = now()
		WHERE id = $1 AND status = 'draft'`, meetingID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotDraft
	}
	return nil
}

// Finish закрывает голосование: после этого голоса не принимаются.
func (s *Store) Finish(ctx context.Context, meetingID int) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE votings SET status = 'finished' WHERE id = $1 AND status = 'active'`, meetingID)
	return err
}

func inviteToken() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
