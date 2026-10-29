package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/mahm0udnasr/tasker/internal/domain"
	"github.com/mahm0udnasr/tasker/internal/store"
)

const projectColumns = `id, name, color, icon, sort_order, created_at, updated_at, deleted_at`

// queryProjects runs a SELECT with a WHERE clause built from constant
// fragments in this package; values are always bound as args.
func (s *Store) queryProjects(ctx context.Context, where string, args ...any) ([]domain.Project, error) {
	q := "SELECT " + projectColumns + " FROM projects WHERE " + where + " ORDER BY sort_order, lower(name)" //nolint:gosec // see above
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("query projects: %w", err)
	}
	defer rows.Close()
	var out []domain.Project
	for rows.Next() {
		var (
			p                domain.Project
			created, updated int64
			deleted          sql.NullInt64
		)
		if err := rows.Scan(&p.ID, &p.Name, &p.Color, &p.Icon, &p.SortOrder, &created, &updated, &deleted); err != nil {
			return nil, err
		}
		p.CreatedAt, p.UpdatedAt, p.DeletedAt = fromMS(created), fromMS(updated), fromNullMS(deleted)
		out = append(out, p)
	}
	return out, rows.Err()
}

// ListProjects implements store.Store.
func (s *Store) ListProjects(ctx context.Context) ([]domain.Project, error) {
	return s.queryProjects(ctx, "deleted_at IS NULL")
}

// GetProject implements store.Store.
func (s *Store) GetProject(ctx context.Context, idOrPrefix string) (domain.Project, error) {
	pat, err := prefixPattern(idOrPrefix)
	if err != nil {
		return domain.Project{}, err
	}
	ps, err := s.queryProjects(ctx, "deleted_at IS NULL AND id LIKE ?", pat)
	if err != nil {
		return domain.Project{}, err
	}
	switch len(ps) {
	case 0:
		return domain.Project{}, notFound("project", idOrPrefix)
	case 1:
		return ps[0], nil
	default:
		return domain.Project{}, fmt.Errorf("project %q: %w", idOrPrefix, store.ErrAmbiguousID)
	}
}

// GetProjectByName implements store.Store.
func (s *Store) GetProjectByName(ctx context.Context, name string) (domain.Project, error) {
	ps, err := s.queryProjects(ctx, "deleted_at IS NULL AND name = ? COLLATE NOCASE", strings.TrimSpace(name))
	if err != nil {
		return domain.Project{}, err
	}
	if len(ps) == 0 {
		return domain.Project{}, notFound("project", name)
	}
	return ps[0], nil
}

func (s *Store) writeProject(ctx context.Context, q queryer, p *domain.Project, overwrite bool) error {
	if err := p.Validate(); err != nil {
		return err
	}
	stmt := `INSERT INTO projects (` + projectColumns + `) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
	if overwrite {
		stmt += ` ON CONFLICT(id) DO UPDATE SET name = excluded.name, color = excluded.color,
			icon = excluded.icon, sort_order = excluded.sort_order, created_at = excluded.created_at,
			updated_at = excluded.updated_at, deleted_at = excluded.deleted_at`
	}
	_, err := q.ExecContext(ctx, stmt, p.ID, p.Name, p.Color, p.Icon, p.SortOrder,
		toMS(p.CreatedAt), toMS(p.UpdatedAt), toNullMS(p.DeletedAt))
	if isUniqueViolation(err) {
		return fmt.Errorf("project %q: %w", p.Name, store.ErrDuplicateName)
	}
	if err != nil {
		return fmt.Errorf("write project: %w", err)
	}
	return nil
}

// CreateProject implements store.Store.
func (s *Store) CreateProject(ctx context.Context, p *domain.Project) error {
	if p.ID == "" {
		p.ID = domain.NewID()
	}
	return s.writeProject(ctx, s.db, p, false)
}

// UpdateProject implements store.Store.
func (s *Store) UpdateProject(ctx context.Context, p *domain.Project) error {
	if err := p.Validate(); err != nil {
		return err
	}
	res, err := s.db.ExecContext(ctx, `UPDATE projects SET name = ?, color = ?, icon = ?, sort_order = ?,
		updated_at = ? WHERE id = ? AND deleted_at IS NULL`,
		p.Name, p.Color, p.Icon, p.SortOrder, toMS(p.UpdatedAt), p.ID)
	if isUniqueViolation(err) {
		return fmt.Errorf("project %q: %w", p.Name, store.ErrDuplicateName)
	}
	if err != nil {
		return fmt.Errorf("update project: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return notFound("project", p.ID)
	}
	return nil
}

// DeleteProject implements store.Store.
func (s *Store) DeleteProject(ctx context.Context, id string, now time.Time) error {
	return s.inTx(ctx, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, `UPDATE projects SET deleted_at = ?, updated_at = ?
			WHERE id = ? AND deleted_at IS NULL`, toMS(now), toMS(now), id)
		if err != nil {
			return fmt.Errorf("delete project: %w", err)
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return notFound("project", id)
		}
		_, err = tx.ExecContext(ctx, `UPDATE tasks SET project_id = NULL, updated_at = ? WHERE project_id = ?`,
			toMS(now), id)
		return err
	})
}

// RestoreProjects implements store.Store.
func (s *Store) RestoreProjects(ctx context.Context, ps []domain.Project) error {
	return s.inTx(ctx, func(tx *sql.Tx) error {
		for i := range ps {
			if err := s.writeProject(ctx, tx, &ps[i], true); err != nil {
				return err
			}
		}
		return nil
	})
}
