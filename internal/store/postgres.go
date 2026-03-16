package store

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"time"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

type Message struct {
	ID        int64     `json:"id"`
	Text      string    `json:"text"`
	CreatedAt time.Time `json:"created_at"`
}

type PostgresMessageStore struct {
	db *sql.DB
}

func NewPostgresMessageStore(db *sql.DB) *PostgresMessageStore {
	return &PostgresMessageStore{db: db}
}

func (s *PostgresMessageStore) Migrate(ctx context.Context) error {
	query, err := migrationFiles.ReadFile("migrations/001_init.sql")
	if err != nil {
		return fmt.Errorf("read migration: %w", err)
	}

	if _, err := s.db.ExecContext(ctx, string(query)); err != nil {
		return fmt.Errorf("apply migration: %w", err)
	}

	return nil
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
