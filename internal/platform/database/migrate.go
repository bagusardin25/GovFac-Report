package database

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"log/slog"
	"sort"
	"strings"
)

// We apply pending files in name order, one transaction each.
func Migrate(ctx context.Context, db *sql.DB, files fs.FS, logger *slog.Logger) error {
	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version    TEXT        PRIMARY KEY,
		applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
	)`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	names, err := fs.Glob(files, "*.sql")
	if err != nil {
		return fmt.Errorf("list migrations: %w", err)
	}
	sort.Strings(names)

	applied, err := appliedVersions(ctx, db)
	if err != nil {
		return err
	}

	for _, name := range names {
		version := strings.TrimSuffix(name, ".sql")
		if applied[version] {
			continue
		}
		body, err := fs.ReadFile(files, name)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", name, err)
		}
		if err := applyMigration(ctx, db, version, string(body)); err != nil {
			return fmt.Errorf("apply migration %s: %w", name, err)
		}
		logger.Info("migration applied", "version", version)
	}
	return nil
}

func appliedVersions(ctx context.Context, db *sql.DB) (map[string]bool, error) {
	rows, err := db.QueryContext(ctx, `SELECT version FROM schema_migrations`)
	if err != nil {
		return nil, fmt.Errorf("read schema_migrations: %w", err)
	}
	defer rows.Close()

	applied := make(map[string]bool)
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		applied[v] = true
	}
	return applied, rows.Err()
}

func applyMigration(ctx context.Context, db *sql.DB, version, body string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// No query args here, so pgx falls back to the simple protocol and a
	// file can hold several statements.
	if _, err := tx.ExecContext(ctx, body); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations (version) VALUES ($1)`, version); err != nil {
		return err
	}
	return tx.Commit()
}
