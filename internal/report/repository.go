package report

import (
	"context"
	"database/sql"
	"fmt"
)

type Repository interface {
	Create(ctx context.Context, r *Report) error
}

type PostgresRepository struct {
	db *sql.DB
}

func NewPostgresRepository(db *sql.DB) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) Create(ctx context.Context, rep *Report) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	err = tx.QueryRowContext(ctx,
		`INSERT INTO reports (user_id, title, description, photo_key)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, status, created_at`,
		rep.UserID, rep.Title, rep.Description, rep.PhotoKey,
	).Scan(&rep.ID, &rep.Status, &rep.CreatedAt)
	if err != nil {
		return fmt.Errorf("insert report: %w", err)
	}

	for _, tag := range rep.Hashtags {
		// DO UPDATE is a no-op here; it's only there so RETURNING gives us the
		// id of a tag that already exists.
		var tagID int64
		err := tx.QueryRowContext(ctx,
			`INSERT INTO hashtags (name) VALUES ($1)
			 ON CONFLICT (name) DO UPDATE SET name = EXCLUDED.name
			 RETURNING id`,
			tag,
		).Scan(&tagID)
		if err != nil {
			return fmt.Errorf("upsert hashtag %q: %w", tag, err)
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO report_hashtags (report_id, hashtag_id) VALUES ($1, $2)`,
			rep.ID, tagID,
		); err != nil {
			return fmt.Errorf("link hashtag %q: %w", tag, err)
		}
	}

	return tx.Commit()
}
