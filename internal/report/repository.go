package report

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

type Repository interface {
	Create(ctx context.Context, r *Report) error
	ListByUser(ctx context.Context, userID int64, limit, offset int) ([]*Report, int, error)
	FindForUser(ctx context.Context, id, userID int64) (*Report, error)
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
		 RETURNING id, status, created_at, updated_at`,
		rep.UserID, rep.Title, rep.Description, rep.PhotoKey,
	).Scan(&rep.ID, &rep.Status, &rep.CreatedAt, &rep.UpdatedAt)
	if err != nil {
		return fmt.Errorf("insert report: %w", err)
	}

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO report_status_history (report_id, status, created_at) VALUES ($1, $2, $3)`,
		rep.ID, rep.Status, rep.CreatedAt,
	); err != nil {
		return fmt.Errorf("insert status history: %w", err)
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

// Tag names can't contain spaces, so we aggregate them into one string and
// split in Go rather than dealing with Postgres arrays through database/sql.
const selectReport = `
	SELECT r.id, r.user_id, r.title, r.description, r.photo_key, r.status,
	       r.created_at, r.updated_at,
	       COALESCE(string_agg(h.name, ' ' ORDER BY h.name), '')
	FROM reports r
	LEFT JOIN report_hashtags rh ON rh.report_id = r.id
	LEFT JOIN hashtags h ON h.id = rh.hashtag_id`

func (r *PostgresRepository) ListByUser(ctx context.Context, userID int64, limit, offset int) ([]*Report, int, error) {
	var total int
	if err := r.db.QueryRowContext(ctx, `SELECT count(*) FROM reports WHERE user_id = $1`, userID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count reports: %w", err)
	}

	rows, err := r.db.QueryContext(ctx, selectReport+`
		WHERE r.user_id = $1
		GROUP BY r.id
		ORDER BY r.created_at DESC, r.id DESC
		LIMIT $2 OFFSET $3`,
		userID, limit, offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("list reports: %w", err)
	}
	defer rows.Close()

	reports := []*Report{}
	for rows.Next() {
		rep, err := scanReport(rows)
		if err != nil {
			return nil, 0, err
		}
		reports = append(reports, rep)
	}
	return reports, total, rows.Err()
}

func (r *PostgresRepository) FindForUser(ctx context.Context, id, userID int64) (*Report, error) {
	rep, err := scanReport(r.db.QueryRowContext(ctx, selectReport+`
		WHERE r.id = $1 AND r.user_id = $2
		GROUP BY r.id`,
		id, userID,
	))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	rows, err := r.db.QueryContext(ctx,
		`SELECT status, note, created_at
		 FROM report_status_history
		 WHERE report_id = $1
		 ORDER BY created_at, id`,
		id,
	)
	if err != nil {
		return nil, fmt.Errorf("load status history: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var c StatusChange
		if err := rows.Scan(&c.Status, &c.Note, &c.At); err != nil {
			return nil, err
		}
		rep.Timeline = append(rep.Timeline, c)
	}
	return rep, rows.Err()
}

type scanner interface {
	Scan(dest ...any) error
}

func scanReport(s scanner) (*Report, error) {
	var rep Report
	var tags string
	err := s.Scan(&rep.ID, &rep.UserID, &rep.Title, &rep.Description, &rep.PhotoKey, &rep.Status,
		&rep.CreatedAt, &rep.UpdatedAt, &tags)
	if err != nil {
		return nil, err
	}
	rep.Hashtags = strings.Fields(tags)
	return &rep, nil
}
