package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mahm0udnasr/tasker/internal/domain"
	"github.com/mahm0udnasr/tasker/internal/store"
)

// taskColumns selects a task plus its derived subtask counts and tag list.
// Tag names cannot contain commas (domain.NormalizeTag), so group_concat with
// ',' is a safe encoding.
const taskColumns = `
	t.id, COALESCE(t.parent_id, ''), COALESCE(t.project_id, ''), t.title, t.description,
	t.status, t.priority, t.due_at, t.due_has_time, t.sort_order,
	t.created_at, t.updated_at, t.completed_at, t.deleted_at,
	(SELECT COUNT(*) FROM tasks s WHERE s.parent_id = t.id AND s.deleted_at IS NULL),
	(SELECT COUNT(*) FROM tasks s WHERE s.parent_id = t.id AND s.deleted_at IS NULL AND s.status = 'done'),
	COALESCE((SELECT group_concat(g.name, ',') FROM task_tags tt JOIN tags g ON g.id = tt.tag_id
	          WHERE tt.task_id = t.id), '')`

func scanTask(sc interface{ Scan(...any) error }) (domain.Task, error) {
	var (
		t                       domain.Task
		status                  string
		due, completed, deleted sql.NullInt64
		created, updated        int64
		hasTime                 bool
		tags                    string
	)
	err := sc.Scan(&t.ID, &t.ParentID, &t.ProjectID, &t.Title, &t.Description,
		&status, &t.Priority, &due, &hasTime, &t.SortOrder,
		&created, &updated, &completed, &deleted,
		&t.SubtasksTotal, &t.SubtasksDone, &tags)
	if err != nil {
		return t, err
	}
	t.Status = domain.Status(status)
	t.Due = fromNullMS(due)
	t.DueHasTime = hasTime
	t.CreatedAt, t.UpdatedAt = fromMS(created), fromMS(updated)
	t.CompletedAt, t.DeletedAt = fromNullMS(completed), fromNullMS(deleted)
	if tags != "" {
		t.Tags = strings.Split(tags, ",")
		sort.Strings(t.Tags)
	}
	return t, nil
}

func (s *Store) queryTasks(ctx context.Context, q queryer, where string, args []any, order string) ([]domain.Task, error) {
	query := "SELECT " + taskColumns + " FROM tasks t"
	if where != "" {
		query += " WHERE " + where
	}
	if order != "" {
		query += " ORDER BY " + order
	}
	rows, err := q.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query tasks: %w", err)
	}
	defer rows.Close()
	var out []domain.Task
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, fmt.Errorf("scan task: %w", err)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// whereFor translates a domain.Filter into SQL. It must agree with
// domain.Filter.Match; the store tests check both on the same fixtures.
func whereFor(f domain.Filter) (string, []any) {
	now := f.Now
	if now.IsZero() {
		now = time.Now()
	}
	today, tomorrow := f.Bounds()
	conds := []string{"t.deleted_at IS NULL"}
	var args []any
	add := func(c string, a ...any) {
		conds = append(conds, c)
		args = append(args, a...)
	}

	switch {
	case f.ParentID != "":
		add("t.parent_id = ?", f.ParentID)
	case !f.IncludeSubtasks:
		add("t.parent_id IS NULL")
	}
	if f.ProjectID != "" {
		add("t.project_id = ?", f.ProjectID)
	}
	if f.Tag != "" {
		tag, err := domain.NormalizeTag(f.Tag)
		if err != nil {
			tag = f.Tag // an invalid tag simply matches nothing
		}
		add(`EXISTS (SELECT 1 FROM task_tags tt JOIN tags g ON g.id = tt.tag_id
		             WHERE tt.task_id = t.id AND g.name = ?)`, tag)
	}

	open := func() {
		if !f.IncludeDone {
			add("t.status <> 'done'")
		}
	}
	switch f.View {
	case domain.ViewInbox:
		open()
		add("t.project_id IS NULL")
	case domain.ViewToday:
		open()
		add("t.due_at IS NOT NULL AND t.due_at < ?", toMS(tomorrow))
	case domain.ViewUpcoming:
		open()
		add("t.due_at >= ?", toMS(tomorrow))
	case domain.ViewOverdue:
		add(overdueSQL, toMS(now), toMS(today))
	case domain.ViewCompleted:
		add("t.status = 'done'")
	default:
		open()
	}
	return strings.Join(conds, " AND "), args
}

// overdueSQL mirrors domain.Task.IsOverdue: timed tasks are overdue once the
// moment passes, all-day tasks once their day has ended. Args: now, today.
const overdueSQL = `t.status <> 'done' AND t.due_at IS NOT NULL AND
	((t.due_has_time = 1 AND t.due_at < ?) OR (t.due_has_time = 0 AND t.due_at < ?))`

func orderFor(k domain.SortKey) string {
	const tail = "t.sort_order, t.created_at"
	switch k {
	case domain.SortDue:
		return "t.due_at IS NULL, t.due_at, t.priority DESC, " + tail
	case domain.SortPriority:
		return "t.priority DESC, " + tail
	case domain.SortCreated:
		return "t.created_at DESC, t.sort_order"
	case domain.SortTitle:
		return "lower(t.title), " + tail
	case domain.SortCompleted:
		return "t.completed_at DESC, " + tail
	default:
		return tail
	}
}

// ListTasks implements store.Store.
func (s *Store) ListTasks(ctx context.Context, f domain.Filter, k domain.SortKey) ([]domain.Task, error) {
	where, args := whereFor(f)
	return s.queryTasks(ctx, s.db, where, args, orderFor(k))
}

// GetTask implements store.Store.
func (s *Store) GetTask(ctx context.Context, idOrPrefix string) (domain.Task, error) {
	return s.getTask(ctx, s.db, idOrPrefix)
}

func (s *Store) getTask(ctx context.Context, q queryer, idOrPrefix string) (domain.Task, error) {
	pat, err := prefixPattern(idOrPrefix)
	if err != nil {
		return domain.Task{}, err
	}
	ts, err := s.queryTasks(ctx, q, "t.deleted_at IS NULL AND t.id LIKE ?", []any{pat}, "t.id LIMIT 2")
	if err != nil {
		return domain.Task{}, err
	}
	switch len(ts) {
	case 0:
		return domain.Task{}, notFound("task", idOrPrefix)
	case 1:
		return ts[0], nil
	default:
		return domain.Task{}, fmt.Errorf("task %q: %w", idOrPrefix, store.ErrAmbiguousID)
	}
}

// checkRelations validates the parent and project references of t and, for
// subtasks, forces the project to match the parent's.
func (s *Store) checkRelations(ctx context.Context, tx *sql.Tx, t *domain.Task) error {
	if t.ParentID != "" {
		var grand sql.NullString
		var project string
		err := tx.QueryRowContext(ctx,
			`SELECT parent_id, COALESCE(project_id, '') FROM tasks WHERE id = ? AND deleted_at IS NULL`,
			t.ParentID).Scan(&grand, &project)
		switch {
		case isNoRows(err):
			return fmt.Errorf("parent %q: %w", t.ParentID, store.ErrInvalidParent)
		case err != nil:
			return err
		case grand.Valid:
			return fmt.Errorf("%w: %w", store.ErrInvalidParent, domain.ErrNestedSubtask)
		}
		var children int
		if err := tx.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM tasks WHERE parent_id = ? AND deleted_at IS NULL`, t.ID).Scan(&children); err != nil {
			return err
		}
		if children > 0 {
			return fmt.Errorf("%w: a task with subtasks cannot become a subtask", store.ErrInvalidParent)
		}
		t.ProjectID = project
	}
	if t.ProjectID != "" {
		var n int
		if err := tx.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM projects WHERE id = ? AND deleted_at IS NULL`, t.ProjectID).Scan(&n); err != nil {
			return err
		}
		if n == 0 {
			return notFound("project", t.ProjectID)
		}
	}
	return nil
}

// CreateTask implements store.Store.
func (s *Store) CreateTask(ctx context.Context, t *domain.Task) error {
	if t.ID == "" {
		t.ID = domain.NewID()
	}
	if err := t.Validate(); err != nil {
		return err
	}
	if err := t.SetTags(t.Tags); err != nil {
		return err
	}
	return s.inTx(ctx, func(tx *sql.Tx) error {
		if err := s.checkRelations(ctx, tx, t); err != nil {
			return err
		}
		if t.SortOrder == 0 {
			if err := tx.QueryRowContext(ctx,
				`SELECT COALESCE(MAX(sort_order), 0) + 1 FROM tasks`).Scan(&t.SortOrder); err != nil {
				return err
			}
		}
		if err := upsertTask(ctx, tx, t, false); err != nil {
			return err
		}
		return setTaskTags(ctx, tx, t.ID, t.Tags, t.UpdatedAt)
	})
}

// UpdateTask implements store.Store.
func (s *Store) UpdateTask(ctx context.Context, t *domain.Task) error {
	if err := t.Validate(); err != nil {
		return err
	}
	if err := t.SetTags(t.Tags); err != nil {
		return err
	}
	return s.inTx(ctx, func(tx *sql.Tx) error {
		if err := s.checkRelations(ctx, tx, t); err != nil {
			return err
		}
		res, err := tx.ExecContext(ctx, `UPDATE tasks SET
			parent_id = ?, project_id = ?, title = ?, description = ?, status = ?, priority = ?,
			due_at = ?, due_has_time = ?, sort_order = ?, updated_at = ?, completed_at = ?
			WHERE id = ? AND deleted_at IS NULL`,
			nullStr(t.ParentID), nullStr(t.ProjectID), t.Title, t.Description, string(t.Status), t.Priority,
			toNullMS(t.Due), t.DueHasTime, t.SortOrder, toMS(t.UpdatedAt), toNullMS(t.CompletedAt), t.ID)
		if err != nil {
			return fmt.Errorf("update task: %w", err)
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return notFound("task", t.ID)
		}
		// Subtasks always live in their parent's project.
		if _, err := tx.ExecContext(ctx, `UPDATE tasks SET project_id = ?, updated_at = ?
			WHERE parent_id = ? AND deleted_at IS NULL AND COALESCE(project_id, '') <> ?`,
			nullStr(t.ProjectID), toMS(t.UpdatedAt), t.ID, t.ProjectID); err != nil {
			return err
		}
		return setTaskTags(ctx, tx, t.ID, t.Tags, t.UpdatedAt)
	})
}

// upsertTask writes every column. With overwrite=false it is a plain insert.
func upsertTask(ctx context.Context, tx *sql.Tx, t *domain.Task, overwrite bool) error {
	q := `INSERT INTO tasks (id, parent_id, project_id, title, description, status, priority,
		due_at, due_has_time, sort_order, created_at, updated_at, completed_at, deleted_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	if overwrite {
		q += ` ON CONFLICT(id) DO UPDATE SET parent_id = excluded.parent_id,
			project_id = excluded.project_id, title = excluded.title, description = excluded.description,
			status = excluded.status, priority = excluded.priority, due_at = excluded.due_at,
			due_has_time = excluded.due_has_time, sort_order = excluded.sort_order,
			created_at = excluded.created_at, updated_at = excluded.updated_at,
			completed_at = excluded.completed_at, deleted_at = excluded.deleted_at`
	}
	_, err := tx.ExecContext(ctx, q, t.ID, nullStr(t.ParentID), nullStr(t.ProjectID), t.Title,
		t.Description, string(t.Status), t.Priority, toNullMS(t.Due), t.DueHasTime, t.SortOrder,
		toMS(t.CreatedAt), toMS(t.UpdatedAt), toNullMS(t.CompletedAt), toNullMS(t.DeletedAt))
	if err != nil {
		return fmt.Errorf("write task: %w", err)
	}
	return nil
}

// setTaskTags replaces the task's tag links, creating tag rows on demand.
func setTaskTags(ctx context.Context, tx *sql.Tx, taskID string, tags []string, now time.Time) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM task_tags WHERE task_id = ?`, taskID); err != nil {
		return err
	}
	for _, name := range tags {
		var id string
		err := tx.QueryRowContext(ctx, `SELECT id FROM tags WHERE name = ? AND deleted_at IS NULL`, name).Scan(&id)
		if isNoRows(err) {
			id = domain.NewID()
			_, err = tx.ExecContext(ctx, `INSERT INTO tags (id, name, created_at, updated_at) VALUES (?, ?, ?, ?)`,
				id, name, toMS(now), toMS(now))
		}
		if err != nil {
			return fmt.Errorf("tag %q: %w", name, err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO task_tags (task_id, tag_id) VALUES (?, ?)`, taskID, id); err != nil {
			return fmt.Errorf("tag %q: %w", name, err)
		}
	}
	return nil
}

// DeleteTask implements store.Store.
func (s *Store) DeleteTask(ctx context.Context, id string, now time.Time) error {
	res, err := s.db.ExecContext(ctx, `UPDATE tasks SET deleted_at = ?, updated_at = ?
		WHERE (id = ? OR parent_id = ?) AND deleted_at IS NULL`, toMS(now), toMS(now), id, id)
	if err != nil {
		return fmt.Errorf("delete task: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return notFound("task", id)
	}
	return nil
}

// RestoreTasks implements store.Store.
func (s *Store) RestoreTasks(ctx context.Context, ts []domain.Task) error {
	ordered := make([]domain.Task, len(ts))
	copy(ordered, ts)
	// Parents first so the parent_id foreign key is satisfied.
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].ParentID == "" && ordered[j].ParentID != "" })
	return s.inTx(ctx, func(tx *sql.Tx) error {
		for i := range ordered {
			t := &ordered[i]
			if err := t.Validate(); err != nil {
				return fmt.Errorf("task %s: %w", domain.ShortID(t.ID), err)
			}
			if err := t.SetTags(t.Tags); err != nil {
				return err
			}
			if err := upsertTask(ctx, tx, t, true); err != nil {
				return err
			}
			if err := setTaskTags(ctx, tx, t.ID, t.Tags, t.UpdatedAt); err != nil {
				return err
			}
		}
		return nil
	})
}

// ChangesSince implements store.Store.
func (s *Store) ChangesSince(ctx context.Context, since time.Time) (store.ChangeSet, error) {
	var cs store.ChangeSet
	var err error
	cs.Tasks, err = s.queryTasks(ctx, s.db, "t.updated_at > ?", []any{toMS(since)}, "t.updated_at")
	if err != nil {
		return cs, err
	}
	cs.Projects, err = s.queryProjects(ctx, "updated_at > ?", toMS(since))
	return cs, err
}
