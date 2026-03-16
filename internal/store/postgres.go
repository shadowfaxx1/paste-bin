package store

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"time"
)

// ErrNotFound is returned by GetKV when the key does not exist.
var ErrNotFound = errors.New("not found")

//go:embed migrations/*.sql
var migrationFiles embed.FS

type Message struct {
	ID        int64     `json:"id"`
	Text      string    `json:"text"`
	CreatedAt time.Time `json:"created_at"`
}

type KVEntry struct {
	Key       string    `json:"key"`
	Value     string    `json:"value"`
	UpdatedAt time.Time `json:"updated_at"`
}

type PostgresMessageStore struct {
	db *sql.DB
}

func NewPostgresMessageStore(db *sql.DB) *PostgresMessageStore {
	return &PostgresMessageStore{db: db}
}

func (s *PostgresMessageStore) Migrate(ctx context.Context) error {
	migrations := []string{
		"migrations/001_init.sql",
		"migrations/002_kv.sql",
	}
	for _, name := range migrations {
		query, err := migrationFiles.ReadFile(name)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", name, err)
		}
		if _, err := s.db.ExecContext(ctx, string(query)); err != nil {
			return fmt.Errorf("apply migration %s: %w", name, err)
		}
	}
	return nil
}

func (s *PostgresMessageStore) SetKV(ctx context.Context, key, value string) (KVEntry, error) {
	const query = `
		INSERT INTO kv (key, value, updated_at)
		VALUES ($1, $2, NOW())
		ON CONFLICT (key) DO UPDATE
			SET value = EXCLUDED.value, updated_at = NOW()
		RETURNING key, value, updated_at;
	`
	var e KVEntry
	err := s.db.QueryRowContext(ctx, query, key, value).Scan(&e.Key, &e.Value, &e.UpdatedAt)
	return e, err
}

func (s *PostgresMessageStore) GetKV(ctx context.Context, key string) (KVEntry, error) {
	const query = `SELECT key, value, updated_at FROM kv WHERE key = $1;`
	var e KVEntry
	err := s.db.QueryRowContext(ctx, query, key).Scan(&e.Key, &e.Value, &e.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return KVEntry{}, ErrNotFound
	}
	return e, err
}

func (s *PostgresMessageStore) CreateMessage(ctx context.Context, text string) (Message, error) {
	const query = `
		INSERT INTO messages (text)
		VALUES ($1)
		RETURNING id, text, created_at;
	`

	var message Message
	err := s.db.QueryRowContext(ctx, query, text).Scan(&message.ID, &message.Text, &message.CreatedAt)
	if err != nil {
		return Message{}, err
	}

	return message, nil
}

func (s *PostgresMessageStore) ListMessages(ctx context.Context, limit int) ([]Message, error) {
	const query = `
		SELECT id, text, created_at
		FROM messages
		ORDER BY created_at DESC, id DESC
		LIMIT $1;
	`

	rows, err := s.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	messages := make([]Message, 0, limit)
	for rows.Next() {
		var message Message
		if err := rows.Scan(&message.ID, &message.Text, &message.CreatedAt); err != nil {
			return nil, err
		}
		messages = append(messages, message)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return messages, nil
}

func (s *PostgresMessageStore) Ping(ctx context.Context) error {
	return s.db.PingContext(ctx)
}
