package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/mahm0udnasr/tasker/internal/domain"
	"github.com/mahm0udnasr/tasker/internal/store"
)

const habitColumns = `id, name, created_at, updated_at, deleted_at`

// queryHabits runs a SELECT with a WHERE clause built from constant
// fragments in this package; values are always bound as args.
func (s *Store) queryHabits(ctx context.Context, where string, args ...any) ([]domain.Habit, error) {
	q := "SELECT " + habitColumns + " FROM habits WHERE " + where + " ORDER BY created_at, lower(name)" //nolint:gosec // see above
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("query habits: %w", err)
	}
	defer rows.Close()
	var out []domain.Habit
	for rows.Next() {
		var (
			h                domain.Habit
			created, updated int64
			deleted          sql.NullInt64
		)
		if err := rows.Scan(&h.ID, &h.Name, &created, &updated, &deleted); err != nil {
			return nil, err
		}
		h.CreatedAt, h.UpdatedAt, h.DeletedAt = fromMS(created), fromMS(updated), fromNullMS(deleted)
		out = append(out, h)
	}
	return out, rows.Err()
}

// ListHabits implements store.Store.
func (s *Store) ListHabits(ctx context.Context) ([]domain.Habit, error) {
	return s.queryHabits(ctx, "deleted_at IS NULL")
}

// GetHabit implements store.Store.
func (s *Store) GetHabit(ctx context.Context, idOrPrefix string) (domain.Habit, error) {
	pat, err := prefixPattern(idOrPrefix)
	if err != nil {
		return domain.Habit{}, err
	}
	hs, err := s.queryHabits(ctx, "deleted_at IS NULL AND id LIKE ?", pat)
	if err != nil {
		return domain.Habit{}, err
	}
	switch len(hs) {
	case 0:
		return domain.Habit{}, notFound("habit", idOrPrefix)
	case 1:
		return hs[0], nil
	default:
		return domain.Habit{}, fmt.Errorf("habit %q: %w", idOrPrefix, store.ErrAmbiguousID)
	}
}

func writeHabit(ctx context.Context, q queryer, h *domain.Habit, overwrite bool) error {
	if err := h.Validate(); err != nil {
		return err
	}
	stmt := `INSERT INTO habits (` + habitColumns + `) VALUES (?, ?, ?, ?, ?)`
	if overwrite {
		stmt += ` ON CONFLICT(id) DO UPDATE SET name = excluded.name, created_at = excluded.created_at,
			updated_at = excluded.updated_at, deleted_at = excluded.deleted_at`
	}
	_, err := q.ExecContext(ctx, stmt, h.ID, h.Name, toMS(h.CreatedAt), toMS(h.UpdatedAt), toNullMS(h.DeletedAt))
	if isUniqueViolation(err) {
		return fmt.Errorf("habit %q: %w", h.Name, store.ErrDuplicateName)
	}
	if err != nil {
		return fmt.Errorf("write habit: %w", err)
	}
	return nil
}

// CreateHabit implements store.Store.
func (s *Store) CreateHabit(ctx context.Context, h *domain.Habit) error {
	if h.ID == "" {
		h.ID = domain.NewID()
	}
	return writeHabit(ctx, s.db, h, false)
}

// UpdateHabit implements store.Store.
func (s *Store) UpdateHabit(ctx context.Context, h *domain.Habit) error {
	if err := h.Validate(); err != nil {
		return err
	}
	res, err := s.db.ExecContext(ctx, `UPDATE habits SET name = ?, updated_at = ?
		WHERE id = ? AND deleted_at IS NULL`, h.Name, toMS(h.UpdatedAt), h.ID)
	if isUniqueViolation(err) {
		return fmt.Errorf("habit %q: %w", h.Name, store.ErrDuplicateName)
	}
	if err != nil {
		return fmt.Errorf("update habit: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return notFound("habit", h.ID)
	}
	return nil
}

// DeleteHabit implements store.Store.
func (s *Store) DeleteHabit(ctx context.Context, id string, now time.Time) error {
	res, err := s.db.ExecContext(ctx, `UPDATE habits SET deleted_at = ?, updated_at = ?
		WHERE id = ? AND deleted_at IS NULL`, toMS(now), toMS(now), id)
	if err != nil {
		return fmt.Errorf("delete habit: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return notFound("habit", id)
	}
	return nil
}

// RestoreHabits implements store.Store.
func (s *Store) RestoreHabits(ctx context.Context, hs []domain.Habit) error {
	return s.inTx(ctx, func(tx *sql.Tx) error {
		for i := range hs {
			if err := writeHabit(ctx, tx, &hs[i], true); err != nil {
				return err
			}
		}
		return nil
	})
}

// SetHabitDone implements store.Store. Marking a day undone tombstones its
// log row; marking it done again revives the row.
func (s *Store) SetHabitDone(ctx context.Context, habitID, day string, done bool, now time.Time) error {
	if _, err := domain.ParseDay(day); err != nil {
		return err
	}
	return s.inTx(ctx, func(tx *sql.Tx) error {
		var n int
		if err := tx.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM habits WHERE id = ? AND deleted_at IS NULL`, habitID).Scan(&n); err != nil {
			return err
		}
		if n == 0 {
			return notFound("habit", habitID)
		}
		var deleted sql.NullInt64 // NULL: done
		if !done {
			deleted = toNullMS(&now)
		}
		_, err := tx.ExecContext(ctx, `INSERT INTO habit_logs (habit_id, day, created_at, updated_at, deleted_at)
			VALUES (?, ?, ?, ?, ?)
			ON CONFLICT(habit_id, day) DO UPDATE SET updated_at = excluded.updated_at, deleted_at = excluded.deleted_at`,
			habitID, day, toMS(now), toMS(now), deleted)
		if err != nil {
			return fmt.Errorf("set habit done: %w", err)
		}
		return nil
	})
}

// ListHabitLogs implements store.Store.
func (s *Store) ListHabitLogs(ctx context.Context) ([]domain.HabitLog, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT l.habit_id, l.day, l.created_at, l.updated_at
		FROM habit_logs l JOIN habits h ON h.id = l.habit_id
		WHERE l.deleted_at IS NULL AND h.deleted_at IS NULL
		ORDER BY l.habit_id, l.day`)
	if err != nil {
		return nil, fmt.Errorf("query habit logs: %w", err)
	}
	defer rows.Close()
	var out []domain.HabitLog
	for rows.Next() {
		var (
			l                domain.HabitLog
			created, updated int64
		)
		if err := rows.Scan(&l.HabitID, &l.Day, &created, &updated); err != nil {
			return nil, err
		}
		l.CreatedAt, l.UpdatedAt = fromMS(created), fromMS(updated)
		out = append(out, l)
	}
	return out, rows.Err()
}
