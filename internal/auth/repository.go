package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

// The service only depends on this interface, so we can test it without Postgres.
type Repository interface {
	CreateUser(ctx context.Context, u *User) error
	FindUserByEmail(ctx context.Context, email string) (*User, error)
	CreateSession(ctx context.Context, s *Session) error
	FindUserBySession(ctx context.Context, tokenHash string, now time.Time) (*User, error)
	DeleteSession(ctx context.Context, tokenHash string) error
	ReplacePasswordReset(ctx context.Context, pr *PasswordReset) error
	ResetPassword(ctx context.Context, tokenHash, passwordHash string, now time.Time) error
}

type PostgresRepository struct {
	db *sql.DB
}

func NewPostgresRepository(db *sql.DB) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) CreateUser(ctx context.Context, u *User) error {
	err := r.db.QueryRowContext(ctx,
		`INSERT INTO users (name, email, password_hash, role)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, created_at`,
		u.Name, u.Email, u.PasswordHash, u.Role,
	).Scan(&u.ID, &u.CreatedAt)
	if isUniqueViolation(err) {
		return ErrEmailTaken
	}
	if err != nil {
		return fmt.Errorf("insert user: %w", err)
	}
	return nil
}

func (r *PostgresRepository) FindUserByEmail(ctx context.Context, email string) (*User, error) {
	var u User
	err := r.db.QueryRowContext(ctx,
		`SELECT id, name, email, password_hash, role, created_at
		 FROM users
		 WHERE lower(email) = lower($1)`,
		email,
	).Scan(&u.ID, &u.Name, &u.Email, &u.PasswordHash, &u.Role, &u.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find user by email: %w", err)
	}
	return &u, nil
}

func (r *PostgresRepository) CreateSession(ctx context.Context, s *Session) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO sessions (user_id, token_hash, ip_address, user_agent, expires_at)
		 VALUES ($1, $2, $3, $4, $5)`,
		s.UserID, s.TokenHash, s.IPAddress, s.UserAgent, s.ExpiresAt,
	)
	if err != nil {
		return fmt.Errorf("insert session: %w", err)
	}
	return nil
}

func (r *PostgresRepository) FindUserBySession(ctx context.Context, tokenHash string, now time.Time) (*User, error) {
	var u User
	err := r.db.QueryRowContext(ctx,
		`SELECT u.id, u.name, u.email, u.role, u.created_at
		 FROM sessions s
		 JOIN users u ON u.id = s.user_id
		 WHERE s.token_hash = $1 AND s.expires_at > $2`,
		tokenHash, now,
	).Scan(&u.ID, &u.Name, &u.Email, &u.Role, &u.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrSessionNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find session: %w", err)
	}
	return &u, nil
}

func (r *PostgresRepository) DeleteSession(ctx context.Context, tokenHash string) error {
	if _, err := r.db.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash = $1`, tokenHash); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

// Only the newest reset link per user stays valid.
func (r *PostgresRepository) ReplacePasswordReset(ctx context.Context, pr *PasswordReset) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `DELETE FROM password_resets WHERE user_id = $1`, pr.UserID); err != nil {
		return fmt.Errorf("delete old password resets: %w", err)
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO password_resets (user_id, token_hash, expires_at) VALUES ($1, $2, $3)`,
		pr.UserID, pr.TokenHash, pr.ExpiresAt,
	); err != nil {
		return fmt.Errorf("insert password reset: %w", err)
	}
	return tx.Commit()
}

// DELETE ... RETURNING in the same tx as the update keeps the token single-use
// under concurrent requests. Dropping every session logs the account out everywhere.
func (r *PostgresRepository) ResetPassword(ctx context.Context, tokenHash, passwordHash string, now time.Time) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var userID int64
	err = tx.QueryRowContext(ctx,
		`DELETE FROM password_resets
		 WHERE token_hash = $1 AND expires_at > $2
		 RETURNING user_id`,
		tokenHash, now,
	).Scan(&userID)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrResetTokenInvalid
	}
	if err != nil {
		return fmt.Errorf("consume password reset: %w", err)
	}

	if _, err := tx.ExecContext(ctx,
		`UPDATE users SET password_hash = $1, updated_at = now() WHERE id = $2`,
		passwordHash, userID,
	); err != nil {
		return fmt.Errorf("update password: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM sessions WHERE user_id = $1`, userID); err != nil {
		return fmt.Errorf("delete sessions: %w", err)
	}
	return tx.Commit()
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
