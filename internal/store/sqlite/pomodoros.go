package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/mahm0udnasr/tasker/internal/domain"
	"github.com/mahm0udnasr/tasker/internal/store"
)

const pomodoroColumns = `id, task_id, started_at, length_ms, paused_at, paused_ms, ended_at, completed,
	created_at, updated_at, deleted_at`

func (s *Store) queryPomodoros(ctx context.Context, q queryer, where string, args ...any) ([]domain.Pomodoro, error) {
	query := "SELECT " + pomodoroColumns + " FROM pomodoros WHERE " + where + " ORDER BY started_at" //nolint:gosec // where is a package-internal constant
	rows, err := q.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query pomodoros: %w", err)
	}
	defer rows.Close()
	var out []domain.Pomodoro
	for rows.Next() {
		var (
			p                         domain.Pomodoro
			started, length, pausedMS int64
			created, updated          int64
			pausedAt, ended, deleted  sql.NullInt64
		)
		if err := rows.Scan(&p.ID, &p.TaskID, &started, &length, &pausedAt, &pausedMS, &ended, &p.Completed,
			&created, &updated, &deleted); err != nil {
			return nil, err
		}
		p.Start, p.Length = fromMS(started), time.Duration(length)*time.Millisecond
		p.PausedAt, p.Paused = fromNullMS(pausedAt), time.Duration(pausedMS)*time.Millisecond
		p.EndedAt, p.DeletedAt = fromNullMS(ended), fromNullMS(deleted)
		p.CreatedAt, p.UpdatedAt = fromMS(created), fromMS(updated)
		out = append(out, p)
	}
	return out, rows.Err()
}

func writePomodoro(ctx context.Context, q queryer, p *domain.Pomodoro, overwrite bool) error {
	stmt := `INSERT INTO pomodoros (` + pomodoroColumns + `) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	if overwrite {
		stmt += ` ON CONFLICT(id) DO UPDATE SET task_id = excluded.task_id, started_at = excluded.started_at,
			length_ms = excluded.length_ms, paused_at = excluded.paused_at, paused_ms = excluded.paused_ms,
			ended_at = excluded.ended_at, completed = excluded.completed, created_at = excluded.created_at,
			updated_at = excluded.updated_at, deleted_at = excluded.deleted_at`
	}
	_, err := q.ExecContext(ctx, stmt, p.ID, p.TaskID, toMS(p.Start), p.Length.Milliseconds(),
		toNullMS(p.PausedAt), p.Paused.Milliseconds(), toNullMS(p.EndedAt), p.Completed,
		toMS(p.CreatedAt), toMS(p.UpdatedAt), toNullMS(p.DeletedAt))
	if err != nil {
		return fmt.Errorf("write pomodoro: %w", err)
	}
	return nil
}

// CreatePomodoro implements store.Store.
func (s *Store) CreatePomodoro(ctx context.Context, p *domain.Pomodoro) error {
	if p.ID == "" {
		p.ID = domain.NewID()
	}
	return s.inTx(ctx, func(tx *sql.Tx) error {
		var running int
		if err := tx.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM pomodoros WHERE ended_at IS NULL AND deleted_at IS NULL`).Scan(&running); err != nil {
			return err
		}
		if running > 0 {
			return store.ErrSessionRunning
		}
		var n int
		if err := tx.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM tasks WHERE id = ? AND deleted_at IS NULL`, p.TaskID).Scan(&n); err != nil {
			return err
		}
		if n == 0 {
			return notFound("task", p.TaskID)
		}
		return writePomodoro(ctx, tx, p, false)
	})
}

// UpdatePomodoro implements store.Store.
func (s *Store) UpdatePomodoro(ctx context.Context, p *domain.Pomodoro) error {
	res, err := s.db.ExecContext(ctx, `UPDATE pomodoros SET paused_at = ?, paused_ms = ?, ended_at = ?,
		completed = ?, updated_at = ? WHERE id = ? AND deleted_at IS NULL`,
		toNullMS(p.PausedAt), p.Paused.Milliseconds(), toNullMS(p.EndedAt), p.Completed, toMS(p.UpdatedAt), p.ID)
	if err != nil {
		return fmt.Errorf("update pomodoro: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return notFound("pomodoro", p.ID)
	}
	return nil
}

// ActivePomodoro implements store.Store.
func (s *Store) ActivePomodoro(ctx context.Context) (domain.Pomodoro, error) {
	ps, err := s.queryPomodoros(ctx, s.db, "ended_at IS NULL AND deleted_at IS NULL")
	if err != nil {
		return domain.Pomodoro{}, err
	}
	if len(ps) == 0 {
		return domain.Pomodoro{}, store.ErrNotFound
	}
	return ps[len(ps)-1], nil
}

// ListPomodoros implements store.Store.
func (s *Store) ListPomodoros(ctx context.Context, from, to time.Time) ([]domain.Pomodoro, error) {
	return s.queryPomodoros(ctx, s.db, "deleted_at IS NULL AND started_at >= ? AND started_at < ?", toMS(from), toMS(to))
}

// RestorePomodoros implements store.Store.
func (s *Store) RestorePomodoros(ctx context.Context, ps []domain.Pomodoro) error {
	return s.inTx(ctx, func(tx *sql.Tx) error {
		for i := range ps {
			if err := writePomodoro(ctx, tx, &ps[i], true); err != nil {
				return err
			}
		}
		return nil
	})
}
