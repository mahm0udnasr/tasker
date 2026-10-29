package sqlite

import (
	"context"
	"fmt"
	"time"

	"github.com/mahm0udnasr/tasker/internal/domain"
)

// ListTags implements store.Store.
func (s *Store) ListTags(ctx context.Context) ([]domain.Tag, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT g.name, COUNT(t.id)
		FROM tags g
		JOIN task_tags tt ON tt.tag_id = g.id
		JOIN tasks t ON t.id = tt.task_id AND t.deleted_at IS NULL AND t.status <> 'done'
		WHERE g.deleted_at IS NULL
		GROUP BY g.id
		ORDER BY g.name`)
	if err != nil {
		return nil, fmt.Errorf("list tags: %w", err)
	}
	defer rows.Close()
	var out []domain.Tag
	for rows.Next() {
		var t domain.Tag
		if err := rows.Scan(&t.Name, &t.Count); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// Counts implements store.Store. Badges count top-level tasks only, matching
// what each list shows.
func (s *Store) Counts(ctx context.Context, now time.Time) (domain.Counts, error) {
	today, tomorrow := domain.Filter{Now: now}.Bounds()
	c := domain.Counts{Projects: map[string]int{}, Tags: map[string]int{}}
	err := s.db.QueryRowContext(ctx, `
		SELECT
			COALESCE(SUM(t.status <> 'done' AND t.project_id IS NULL), 0),
			COALESCE(SUM(t.status <> 'done' AND t.due_at < ?), 0),
			COALESCE(SUM(t.status <> 'done' AND t.due_at >= ?), 0),
			COALESCE(SUM(`+overdueSQL+`), 0),
			COALESCE(SUM(t.status = 'done'), 0),
			COALESCE(SUM(t.status = 'done' AND t.completed_at >= ?), 0)
		FROM tasks t
		WHERE t.deleted_at IS NULL AND t.parent_id IS NULL`,
		toMS(tomorrow), toMS(tomorrow), toMS(now), toMS(today), toMS(today),
	).Scan(&c.Inbox, &c.Today, &c.Upcoming, &c.Overdue, &c.Completed, &c.DoneToday)
	if err != nil {
		return c, fmt.Errorf("counts: %w", err)
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT t.project_id, COUNT(*) FROM tasks t
		WHERE t.deleted_at IS NULL AND t.parent_id IS NULL AND t.status <> 'done' AND t.project_id IS NOT NULL
		GROUP BY t.project_id`)
	if err != nil {
		return c, fmt.Errorf("project counts: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var n int
		if err := rows.Scan(&id, &n); err != nil {
			return c, err
		}
		c.Projects[id] = n
	}
	if err := rows.Err(); err != nil {
		return c, err
	}

	tags, err := s.ListTags(ctx)
	if err != nil {
		return c, err
	}
	for _, t := range tags {
		c.Tags[t.Name] = t.Count
	}
	return c, nil
}

// Stats implements store.Store. Completions of subtasks count too: finishing
// a subtask is real progress.
func (s *Store) Stats(ctx context.Context, now time.Time, days int) (domain.Stats, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT completed_at FROM tasks WHERE deleted_at IS NULL AND status = 'done' AND completed_at IS NOT NULL`)
	if err != nil {
		return domain.Stats{}, fmt.Errorf("stats: %w", err)
	}
	defer rows.Close()
	var completions []time.Time
	for rows.Next() {
		var ms int64
		if err := rows.Scan(&ms); err != nil {
			return domain.Stats{}, err
		}
		completions = append(completions, fromMS(ms))
	}
	if err := rows.Err(); err != nil {
		return domain.Stats{}, err
	}

	var total, done int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*), COALESCE(SUM(status = 'done'), 0)
		FROM tasks WHERE deleted_at IS NULL`).Scan(&total, &done); err != nil {
		return domain.Stats{}, fmt.Errorf("stats totals: %w", err)
	}
	return domain.ComputeStats(completions, total, done, days, now), nil
}
