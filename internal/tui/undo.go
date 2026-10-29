package tui

import (
	"context"
	"errors"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/mahm0udnasr/tasker/internal/domain"
	"github.com/mahm0udnasr/tasker/internal/store"
)

// undoLimit bounds memory; older entries are dropped.
const undoLimit = 50

// undoEntry reverses one user action. Snapshots are the records as they
// were before the action; IDs list records the action created. Because
// deletes are soft, restoring a snapshot also brings deleted records back.
type undoEntry struct {
	desc             string
	tasks            []domain.Task
	projects         []domain.Project
	createdTaskIDs   []string
	createdProjectID string
}

type undoStack struct{ entries []undoEntry }

func (s *undoStack) push(e undoEntry) {
	s.entries = append(s.entries, e)
	if len(s.entries) > undoLimit {
		s.entries = s.entries[len(s.entries)-undoLimit:]
	}
}

func (s *undoStack) pop() (undoEntry, bool) {
	if len(s.entries) == 0 {
		return undoEntry{}, false
	}
	e := s.entries[len(s.entries)-1]
	s.entries = s.entries[:len(s.entries)-1]
	return e, true
}

// applyUndo writes the snapshots back. Restored records get UpdatedAt=now so
// a future sync treats the undo as the newest change.
func applyUndo(s store.Store, e undoEntry, now time.Time) tea.Cmd {
	return mutate(func(ctx context.Context) (mutatedMsg, error) {
		for _, id := range e.createdTaskIDs {
			if err := s.DeleteTask(ctx, id, now); err != nil && !errors.Is(err, store.ErrNotFound) {
				return mutatedMsg{}, err
			}
		}
		projects := make([]domain.Project, len(e.projects))
		for i, p := range e.projects {
			p.UpdatedAt = now
			projects[i] = p
		}
		if err := s.RestoreProjects(ctx, projects); err != nil {
			return mutatedMsg{}, err
		}
		tasks := make([]domain.Task, len(e.tasks))
		for i, t := range e.tasks {
			t.UpdatedAt = now
			tasks[i] = t
		}
		if err := s.RestoreTasks(ctx, tasks); err != nil {
			return mutatedMsg{}, err
		}
		// Remove a project the action created, after its tasks moved back.
		if e.createdProjectID != "" {
			if err := s.DeleteProject(ctx, e.createdProjectID, now); err != nil && !errors.Is(err, store.ErrNotFound) {
				return mutatedMsg{}, err
			}
		}
		sel := ""
		if len(e.tasks) > 0 {
			sel = e.tasks[0].ID
		}
		return mutatedMsg{text: "Undid: " + e.desc, selectID: sel, isUndo: true}, nil
	})
}

// snapshotWithSubtasks returns the task and its live subtasks.
func snapshotWithSubtasks(ctx context.Context, s store.Store, id string) ([]domain.Task, error) {
	t, err := s.GetTask(ctx, id)
	if err != nil {
		return nil, err
	}
	subs, err := s.ListTasks(ctx, domain.Filter{ParentID: id, IncludeDone: true}, domain.SortManual)
	if err != nil {
		return nil, err
	}
	return append([]domain.Task{t}, subs...), nil
}
