package notification

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
)

type Repository interface {
	Create(ctx context.Context, n *Notification) error
	ListByUser(ctx context.Context, userID int64, limit, offset int) ([]*Notification, int, error)
}

type PostgresRepository struct {
	db *sql.DB
}

func NewPostgresRepository(db *sql.DB) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) Create(ctx context.Context, n *Notification) error {
	err := r.db.QueryRowContext(ctx,
		`INSERT INTO notifications (user_id, type, title, message, data)
		 VALUES ($1, $2, $3, $4, $5::jsonb)
		 RETURNING id, created_at`,
		n.UserID, n.Type, n.Title, n.Message, string(n.Data),
	).Scan(&n.ID, &n.CreatedAt)
	if err != nil {
		return fmt.Errorf("insert notification: %w", err)
	}
	return nil
}

func (r *PostgresRepository) ListByUser(ctx context.Context, userID int64, limit, offset int) ([]*Notification, int, error) {
	var total int
	if err := r.db.QueryRowContext(ctx, `SELECT count(*) FROM notifications WHERE user_id = $1`, userID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count notifications: %w", err)
	}

	rows, err := r.db.QueryContext(ctx,
		`SELECT id, user_id, type, title, message, data, created_at
		 FROM notifications
		 WHERE user_id = $1
		 ORDER BY created_at DESC, id DESC
		 LIMIT $2 OFFSET $3`,
		userID, limit, offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("list notifications: %w", err)
	}
	defer rows.Close()

	list := []*Notification{}
	for rows.Next() {
		var n Notification
		var data []byte
		if err := rows.Scan(&n.ID, &n.UserID, &n.Type, &n.Title, &n.Message, &data, &n.CreatedAt); err != nil {
			return nil, 0, err
		}
		n.Data = json.RawMessage(data)
		list = append(list, &n)
	}
	return list, total, rows.Err()
}
