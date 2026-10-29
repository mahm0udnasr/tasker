// Package sqlite implements store.Store on SQLite using the pure-Go
// modernc.org/sqlite driver, so tasker builds without CGO.
package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite" // registers the "sqlite" driver

	"github.com/mahm0udnasr/tasker/internal/store"
)

// Store is the SQLite-backed store.Store.
type Store struct {
	db *sql.DB
}

var _ store.Store = (*Store)(nil)

// Open opens (creating if needed) the database at path and applies pending
// migrations.
func Open(ctx context.Context, path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}
	// WAL lets the CLI write while the TUI is open; busy_timeout makes the
	// two wait for each other instead of failing; _txlock=immediate takes
	// the write lock at BEGIN, which avoids SQLITE_BUSY upgrade deadlocks.
	dsn := "file:" + path +
		"?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_txlock=immediate"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("open database %s: %w", path, err)
	}
	if err := migrate(ctx, db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

// Close releases the database.
func (s *Store) Close() error { return s.db.Close() }

// inTx runs fn in a transaction, rolling back on error.
func (s *Store) inTx(ctx context.Context, fn func(*sql.Tx) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

// queryer is satisfied by *sql.DB and *sql.Tx.
type queryer interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

func toMS(t time.Time) int64 { return t.UnixMilli() }

func toNullMS(t *time.Time) sql.NullInt64 {
	if t == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: t.UnixMilli(), Valid: true}
}

func fromMS(ms int64) time.Time { return time.UnixMilli(ms) }

func fromNullMS(n sql.NullInt64) *time.Time {
	if !n.Valid {
		return nil
	}
	t := time.UnixMilli(n.Int64)
	return &t
}

func nullStr(s string) sql.NullString { return sql.NullString{String: s, Valid: s != ""} }

func isUniqueViolation(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed")
}

// prefixPattern validates an ID prefix and turns it into a LIKE pattern.
// IDs are lowercase UUIDs, so anything outside [0-9a-f-] cannot match.
func prefixPattern(p string) (string, error) {
	p = strings.ToLower(strings.TrimSpace(p))
	if len(p) < store.MinIDPrefix {
		return "", fmt.Errorf("id %q is too short (need at least %d characters): %w", p, store.MinIDPrefix, store.ErrNotFound)
	}
	for _, r := range p {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') && r != '-' {
			return "", fmt.Errorf("id %q: %w", p, store.ErrNotFound)
		}
	}
	return p + "%", nil
}

func notFound(kind, id string) error {
	return fmt.Errorf("%s %q: %w", kind, id, store.ErrNotFound)
}

func isNoRows(err error) bool { return errors.Is(err, sql.ErrNoRows) }
