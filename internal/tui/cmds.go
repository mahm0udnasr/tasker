package tui

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/mahm0udnasr/tasker/internal/domain"
	"github.com/mahm0udnasr/tasker/internal/service"
	"github.com/mahm0udnasr/tasker/internal/store"
	"github.com/mahm0udnasr/tasker/internal/tui/common"
	"github.com/mahm0udnasr/tasker/internal/tui/tasklist"
)

// Every store call runs inside a tea.Cmd (off the UI goroutine) with a
// timeout, and reports back with a message. Nothing here touches the model.

const dbTimeout = 5 * time.Second

func dbCtx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), dbTimeout)
}

// --- loads -----------------------------------------------------------------

// tasksLoadedMsg carries a generation number so that results of a stale
// request (the user already moved to another view) are dropped.
type tasksLoadedMsg struct {
	gen  int
	rows []tasklist.Row
	err  error
}

type sidebarLoadedMsg struct {
	projects []domain.Project
	tags     []domain.Tag
	counts   domain.Counts
	err      error
}

func loadTasks(s store.Store, gen int, scope common.Scope, expanded map[string]bool, sort domain.SortKey, now time.Time) tea.Cmd {
	// Copy the map: the command runs concurrently with the UI.
	exp := make(map[string]bool, len(expanded))
	for k, v := range expanded {
		exp[k] = v
	}
	return func() tea.Msg {
		ctx, cancel := dbCtx()
		defer cancel()
		tasks, err := s.ListTasks(ctx, scope.Filter(now), sort)
		if err != nil {
			return tasksLoadedMsg{gen: gen, err: err}
		}
		rows := make([]tasklist.Row, 0, len(tasks))
		for _, t := range tasks {
			rows = append(rows, tasklist.Row{Task: t})
			if !exp[t.ID] || t.SubtasksTotal == 0 {
				continue
			}
			subs, err := s.ListTasks(ctx, domain.Filter{ParentID: t.ID, IncludeDone: true}, domain.SortManual)
			if err != nil {
				return tasksLoadedMsg{gen: gen, err: err}
			}
			for i, st := range subs {
				rows = append(rows, tasklist.Row{Task: st, Depth: 1, Last: i == len(subs)-1})
			}
		}
		return tasksLoadedMsg{gen: gen, rows: rows}
	}
}

func loadSidebar(s store.Store, now time.Time) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := dbCtx()
		defer cancel()
		var msg sidebarLoadedMsg
		if msg.projects, msg.err = s.ListProjects(ctx); msg.err != nil {
			return msg
		}
		if msg.tags, msg.err = s.ListTags(ctx); msg.err != nil {
			return msg
		}
		msg.counts, msg.err = s.Counts(ctx, now)
		return msg
	}
}

// --- mutations ---------------------------------------------------------------

// mutatedMsg reports a successful change; the root reloads and shows text.
type mutatedMsg struct {
	text     string
	selectID string // move the cursor here after reload
	expandID string // expand this parent after reload
}

type errMsg struct{ err error }

func (e errMsg) Error() string { return e.err.Error() }

// mutate runs fn and converts its result into mutatedMsg or errMsg.
func mutate(fn func(ctx context.Context) (mutatedMsg, error)) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := dbCtx()
		defer cancel()
		msg, err := fn(ctx)
		if err != nil {
			return errMsg{err}
		}
		return msg
	}
}

func updateTask(s store.Store, t domain.Task, text string) tea.Cmd {
	return mutate(func(ctx context.Context) (mutatedMsg, error) {
		if err := s.UpdateTask(ctx, &t); err != nil {
			return mutatedMsg{}, err
		}
		return mutatedMsg{text: text, selectID: t.ID}, nil
	})
}

func deleteTask(s store.Store, t domain.Task, now time.Time) tea.Cmd {
	return mutate(func(ctx context.Context) (mutatedMsg, error) {
		if err := s.DeleteTask(ctx, t.ID, now); err != nil {
			return mutatedMsg{}, err
		}
		return mutatedMsg{text: fmt.Sprintf("Deleted “%s”", t.Title)}, nil
	})
}

// saveDraft creates or updates a task from form input, creating the project
// if the name is new.
func saveDraft(s store.Store, d common.TaskDraft, now time.Time) tea.Cmd {
	return mutate(func(ctx context.Context) (mutatedMsg, error) {
		var t domain.Task
		if d.ID != "" {
			var err error
			if t, err = s.GetTask(ctx, d.ID); err != nil {
				return mutatedMsg{}, err
			}
		} else {
			t = domain.NewTask(d.Title, now)
			t.ParentID = d.ParentID
		}
		t.Title, t.Description, t.Priority = d.Title, strings.TrimSpace(d.Description), d.Priority
		due, hasTime, err := domain.ParseDue(d.Due, now)
		if err != nil {
			return mutatedMsg{}, err
		}
		t.SetDue(due, hasTime, now)
		if err := t.SetTags(strings.Split(d.Tags, ",")); err != nil {
			return mutatedMsg{}, err
		}
		var note string
		if t.ParentID == "" {
			if t.ProjectID, note, err = projectID(ctx, s, d.Project, now); err != nil {
				return mutatedMsg{}, err
			}
		}
		t.UpdatedAt = now

		if d.ID != "" {
			err = s.UpdateTask(ctx, &t)
		} else {
			err = s.CreateTask(ctx, &t)
		}
		if err != nil {
			return mutatedMsg{}, err
		}
		verb := "Added"
		if d.ID != "" {
			verb = "Saved"
		}
		return mutatedMsg{text: verb + " “" + t.Title + "”" + note, selectID: t.ID, expandID: t.ParentID}, nil
	})
}

// projectID resolves a project name, creating it when new. note describes
// any creation for the status bar.
func projectID(ctx context.Context, s store.Store, name string, now time.Time) (id, note string, err error) {
	if service.IsInbox(name) {
		return "", "", nil
	}
	p, created, err := service.ResolveProject(ctx, s, strings.TrimSpace(name), now)
	if err != nil {
		return "", "", err
	}
	if created {
		note = " · created project " + p.Name
	}
	return p.ID, note, nil
}

func deleteProject(s store.Store, p domain.Project, now time.Time) tea.Cmd {
	return mutate(func(ctx context.Context) (mutatedMsg, error) {
		if err := s.DeleteProject(ctx, p.ID, now); err != nil {
			return mutatedMsg{}, err
		}
		return mutatedMsg{text: "Deleted project “" + p.Name + "” · tasks moved to Inbox"}, nil
	})
}

func moveTask(s store.Store, t domain.Task, projectName string, now time.Time) tea.Cmd {
	return mutate(func(ctx context.Context) (mutatedMsg, error) {
		id, note, err := projectID(ctx, s, projectName, now)
		if err != nil {
			return mutatedMsg{}, err
		}
		t.ProjectID, t.UpdatedAt = id, now
		if err := s.UpdateTask(ctx, &t); err != nil {
			return mutatedMsg{}, err
		}
		dest := projectName
		if id == "" {
			dest = "Inbox"
		}
		return mutatedMsg{text: "Moved to " + dest + note, selectID: t.ID}, nil
	})
}

// friendly turns store errors into status-bar text.
func friendly(err error) string {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return "database is busy — try again"
	case errors.Is(err, store.ErrNotFound):
		return "that task no longer exists (refreshing)"
	}
	return err.Error()
}
