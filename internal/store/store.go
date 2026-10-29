// Package store defines the persistence boundary. The TUI and CLI depend only
// on the Store interface, so a remote (sync) backend can be dropped in later;
// any implementation must pass the storetest conformance suite.
package store

import (
	"context"
	"errors"
	"time"

	"github.com/mahm0udnasr/tasker/internal/domain"
)

// Errors shared by all backends. Callers match them with errors.Is.
var (
	ErrNotFound      = errors.New("not found")
	ErrAmbiguousID   = errors.New("id prefix matches more than one item")
	ErrDuplicateName = errors.New("name already exists")
	ErrInvalidParent = errors.New("invalid parent task")
)

// MinIDPrefix is the shortest ID prefix accepted by lookups.
const MinIDPrefix = 4

// Store is the persistence API used by the rest of the application.
// All writes set UpdatedAt from the task/project itself, so the caller (which
// owns the clock) decides timestamps.
type Store interface {
	// ListTasks returns tasks matching f, ordered by sort.
	ListTasks(ctx context.Context, f domain.Filter, sort domain.SortKey) ([]domain.Task, error)
	// GetTask resolves a full ID or a unique prefix (≥ MinIDPrefix chars).
	GetTask(ctx context.Context, idOrPrefix string) (domain.Task, error)
	// CreateTask inserts t. A zero SortOrder places it at the end.
	CreateTask(ctx context.Context, t *domain.Task) error
	// UpdateTask overwrites all mutable fields of an existing live task.
	UpdateTask(ctx context.Context, t *domain.Task) error
	// DeleteTask soft-deletes a task and its subtasks.
	DeleteTask(ctx context.Context, id string, now time.Time) error
	// RestoreTasks upserts full task snapshots, including deleted state. It
	// powers undo and JSON import; parents are written before subtasks.
	RestoreTasks(ctx context.Context, ts []domain.Task) error

	ListProjects(ctx context.Context) ([]domain.Project, error)
	GetProject(ctx context.Context, idOrPrefix string) (domain.Project, error)
	// GetProjectByName matches case-insensitively.
	GetProjectByName(ctx context.Context, name string) (domain.Project, error)
	CreateProject(ctx context.Context, p *domain.Project) error
	UpdateProject(ctx context.Context, p *domain.Project) error
	// DeleteProject soft-deletes the project and moves its tasks to the Inbox.
	DeleteProject(ctx context.Context, id string, now time.Time) error
	// RestoreProjects upserts full project snapshots (undo/import).
	RestoreProjects(ctx context.Context, ps []domain.Project) error

	// ListTags returns tags in use by open tasks, with counts.
	ListTags(ctx context.Context) ([]domain.Tag, error)

	// Counts computes sidebar badges relative to now.
	Counts(ctx context.Context, now time.Time) (domain.Counts, error)
	// Stats computes productivity stats for the last `days` days.
	Stats(ctx context.Context, now time.Time, days int) (domain.Stats, error)

	// ChangesSince returns everything (including tombstones) modified after
	// t. It is the hook a future sync engine will build on.
	ChangesSince(ctx context.Context, t time.Time) (ChangeSet, error)

	Close() error
}

// ChangeSet is a batch of modified records.
type ChangeSet struct {
	Tasks    []domain.Task    `json:"tasks"`
	Projects []domain.Project `json:"projects"`
}
