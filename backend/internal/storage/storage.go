// Package storage — репозитории поверх Postgres. Бот, API и воркер ходят
// в базу через него, а не по HTTP друг в друга.
package storage

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"sort"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"oss-max/migrations"
)

var (
	// ErrNotFound — записи нет или она принадлежит другому пользователю.
	// Вызывающему незачем различать эти случаи: ответ одинаковый.
	ErrNotFound = errors.New("не найдено")

	// ErrNotDraft — собрание уже опубликовано, реестр и вопрос менять нельзя.
	ErrNotDraft = errors.New("собрание уже опубликовано")

	// ErrCannotVote — голосование закрыто или помещение не заявлено этим пользователем.
	ErrCannotVote = errors.New("голосовать нельзя")

	// ErrNoRegistry — публиковать нельзя: реестр собственников не загружен,
	// не из чего посчитать площадь дома и кворум.
	ErrNoRegistry = errors.New("реестр собственников не загружен")
)

// querier — общее у пула и транзакции: upsertUser работает с обоими.
type querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type Store struct {
	pool *pgxpool.Pool
}

func Open(ctx context.Context, databaseURL string) (*Store, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("подключение к базе: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("подключение к базе: %w", err)
	}
	return &Store{pool: pool}, nil
}

func (s *Store) Close() {
	s.pool.Close()
}

// Migrate накатывает недостающие миграции по порядку имён файлов.
// Каждая миграция — в своей транзакции: упавшая не оставляет схему наполовину.
func (s *Store) Migrate(ctx context.Context) error {
	conn, err := s.pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()

	// Два процесса, стартующие одновременно, не накатят одно и то же дважды.
	const lockID = 7_431_205
	if _, err := conn.Exec(ctx, `SELECT pg_advisory_lock($1)`, lockID); err != nil {
		return err
	}
	defer conn.Exec(context.Background(), `SELECT pg_advisory_unlock($1)`, lockID)

	_, err = conn.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version    VARCHAR(255) PRIMARY KEY,
		applied_at TIMESTAMPTZ  NOT NULL DEFAULT now()
	)`)
	if err != nil {
		return err
	}

	names, err := fs.Glob(migrations.FS, "*.sql")
	if err != nil {
		return err
	}
	sort.Strings(names)

	for _, name := range names {
		var applied bool
		err := conn.QueryRow(ctx,
			`SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = $1)`, name).Scan(&applied)
		if err != nil {
			return err
		}
		if applied {
			continue
		}

		body, err := fs.ReadFile(migrations.FS, name)
		if err != nil {
			return err
		}

		err = pgx.BeginFunc(ctx, conn, func(tx pgx.Tx) error {
			if _, err := tx.Exec(ctx, string(body)); err != nil {
				return err
			}
			_, err := tx.Exec(ctx, `INSERT INTO schema_migrations (version) VALUES ($1)`, name)
			return err
		})
		if err != nil {
			return fmt.Errorf("миграция %s: %w", name, err)
		}
		log.Printf("миграция %s применена", name)
	}

	return nil
}
