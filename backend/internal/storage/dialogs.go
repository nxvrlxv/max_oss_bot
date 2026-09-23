package storage

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

// User — пользователь MAX, зашедший в бота или мини-приложение.
type User struct {
	MaxID    int64
	Name     string
	Username string
}

// SaveUser создаёт пользователя или обновляет имя. Пустые поля
// не затирают уже известные: в части событий MAX имени нет.
func (s *Store) SaveUser(ctx context.Context, user User) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO users (max_id, full_name, username)
		VALUES ($1, NULLIF($2, ''), NULLIF($3, ''))
		ON CONFLICT (max_id) DO UPDATE SET
			full_name = COALESCE(EXCLUDED.full_name, users.full_name),
			username  = COALESCE(EXCLUDED.username, users.username)`,
		user.MaxID, user.Name, user.Username)
	return err
}

// SaveDialog отмечает, что человек начал переписку и боту можно писать ему первым.
func (s *Store) SaveDialog(ctx context.Context, maxID int64) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO bot_dialogs (max_id) VALUES ($1)
		ON CONFLICT (max_id) DO UPDATE SET is_active = true`,
		maxID)
	return err
}

// CloseDialog — человек остановил бота, писать ему больше нельзя.
func (s *Store) CloseDialog(ctx context.Context, maxID int64) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE bot_dialogs SET is_active = false WHERE max_id = $1`, maxID)
	return err
}

// DialogActive — можно ли отправить человеку личное сообщение, например бланк.
func (s *Store) DialogActive(ctx context.Context, maxID int64) (bool, error) {
	var active bool
	err := s.pool.QueryRow(ctx,
		`SELECT is_active FROM bot_dialogs WHERE max_id = $1`, maxID).Scan(&active)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return active, err
}
