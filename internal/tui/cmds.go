package tui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/mahm0udnasr/tasker/internal/config"
	"github.com/mahm0udnasr/tasker/internal/domain"
	"github.com/mahm0udnasr/tasker/internal/service"
	"github.com/mahm0udnasr/tasker/internal/store"
	"github.com/mahm0udnasr/tasker/internal/tui/common"
	"github.com/mahm0udnasr/tasker/internal/tui/form"
	"github.com/mahm0udnasr/tasker/internal/tui/stats"
	"github.com/mahm0udnasr/tasker/internal/tui/tasklist"
)

// Every store call runs inside a tea.Cmd (off the UI goroutine) with a
// timeout, and reports back with a message. Nothing here touches the model.

const dbTimeout = 5 * time.Second

func dbCtx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), dbTimeout)
}

// --- loads -----------------------------------------------------------------

// listQuery is everything that determines the task list's contents.
type listQuery struct {
	scope    common.Scope
	expanded map[string]bool
	sort     domain.SortKey
	refine   domain.Refinement
}

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

type detailLoadedMsg struct {
	gen      int
	task     *domain.Task
	subtasks []domain.Task
	err      error
}

// searchIndexMsg carries every live task for the search overlay.
type searchIndexMsg struct {
	tasks []domain.Task
	err   error
}

func loadTasks(s store.Store, gen int, q listQuery, now time.Time) tea.Cmd {
	// Copy the map: the command runs concurrently with the UI.
	exp := make(map[string]bool, len(q.expanded))
	for k, v := range q.expanded {
		exp[k] = v
	}
	return func() tea.Msg {
		ctx, cancel := dbCtx()
		defer cancel()
		tasks, err := s.ListTasks(ctx, q.scope.Filter(now), q.sort)
		if err != nil {
			return tasksLoadedMsg{gen: gen, err: err}
		}
		rows := make([]tasklist.Row, 0, len(tasks))
		for _, t := range tasks {
			if !q.refine.Match(t, now) {
				continue
			}
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

func loadDetail(s store.Store, gen int, id string) tea.Cmd {
	return func() tea.Msg {
		if id == "" {
			return detailLoadedMsg{gen: gen}
		}
		ctx, cancel := dbCtx()
		defer cancel()
		t, err := s.GetTask(ctx, id)
		if err != nil {
			return detailLoadedMsg{gen: gen, err: err}
		}
		subs, err := s.ListTasks(ctx, domain.Filter{ParentID: id, IncludeDone: true}, domain.SortManual)
		return detailLoadedMsg{gen: gen, task: &t, subtasks: subs, err: err}
	}
}

func loadSearchIndex(s store.Store) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := dbCtx()
		defer cancel()
		ts, err := s.ListTasks(ctx, domain.Filter{IncludeSubtasks: true, IncludeDone: true}, domain.SortCreated)
		return searchIndexMsg{tasks: ts, err: err}
	}
}

// --- mutations ---------------------------------------------------------------

// mutatedMsg reports a successful change; the root reloads, shows text, and
// pushes undo (if any) onto the undo stack.
type mutatedMsg struct {
	text     string
	selectID string // move the cursor here after reload
	expandID string // expand this parent after reload
	undo     *undoEntry
	isUndo   bool
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

// updateTask saves t, remembering its previous state for undo.
func updateTask(s store.Store, t domain.Task, text string) tea.Cmd {
	return mutate(func(ctx context.Context) (mutatedMsg, error) {
		before, err := s.GetTask(ctx, t.ID)
		if err != nil {
			return mutatedMsg{}, err
		}
		if err := s.UpdateTask(ctx, &t); err != nil {
			return mutatedMsg{}, err
		}
		return mutatedMsg{text: text, selectID: t.ID,
			undo: &undoEntry{desc: text, tasks: []domain.Task{before}}}, nil
	})
}

func deleteTask(s store.Store, t domain.Task, now time.Time) tea.Cmd {
	return mutate(func(ctx context.Context) (mutatedMsg, error) {
		before, err := snapshotWithSubtasks(ctx, s, t.ID)
		if err != nil {
			return mutatedMsg{}, err
		}
		if err := s.DeleteTask(ctx, t.ID, now); err != nil {
			return mutatedMsg{}, err
		}
		text := fmt.Sprintf("Deleted “%s”", t.Title)
		return mutatedMsg{text: text + " · u to undo",
			undo: &undoEntry{desc: text, tasks: before}}, nil
	})
}

// saveDraft creates or updates a task from form input, creating the project
// if the name is new.
func saveDraft(s store.Store, d common.TaskDraft, now time.Time) tea.Cmd {
	return mutate(func(ctx context.Context) (mutatedMsg, error) {
		var t domain.Task
		undo := &undoEntry{}
		if d.ID != "" {
			var err error
			if t, err = s.GetTask(ctx, d.ID); err != nil {
				return mutatedMsg{}, err
			}
			undo.tasks = []domain.Task{t}
		} else {
			t = domain.NewTask(d.Title, now)
			t.ParentID = d.ParentID
			undo.createdTaskIDs = []string{t.ID}
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
		if t.Estimate, err = form.ParseEstimate(d.Estimate); err != nil {
			return mutatedMsg{}, err
		}
		var note string
		if t.ParentID == "" {
			var created bool
			if t.ProjectID, created, err = projectID(ctx, s, d.Project, now); err != nil {
				return mutatedMsg{}, err
			}
			if created {
				note = " · created project " + strings.TrimSpace(d.Project)
				undo.createdProjectID = t.ProjectID
			}
		}
		t.UpdatedAt = now

		verb := "Added"
		if d.ID != "" {
			verb = "Saved"
			err = s.UpdateTask(ctx, &t)
		} else {
			err = s.CreateTask(ctx, &t)
		}
		if err != nil {
			return mutatedMsg{}, err
		}
		undo.desc = verb + " “" + t.Title + "”"
		return mutatedMsg{text: undo.desc + note, selectID: t.ID, expandID: t.ParentID, undo: undo}, nil
	})
}

// projectID resolves a project name, creating it when new.
func projectID(ctx context.Context, s store.Store, name string, now time.Time) (id string, created bool, err error) {
	if service.IsInbox(name) {
		return "", false, nil
	}
	p, created, err := service.ResolveProject(ctx, s, strings.TrimSpace(name), now)
	return p.ID, created, err
}

func moveTask(s store.Store, t domain.Task, projectName string, now time.Time) tea.Cmd {
	return mutate(func(ctx context.Context) (mutatedMsg, error) {
		before, err := s.GetTask(ctx, t.ID)
		if err != nil {
			return mutatedMsg{}, err
		}
		id, created, err := projectID(ctx, s, projectName, now)
		if err != nil {
			return mutatedMsg{}, err
		}
		t.ProjectID, t.UpdatedAt = id, now
		if err := s.UpdateTask(ctx, &t); err != nil {
			return mutatedMsg{}, err
		}
		dest := strings.TrimSpace(projectName)
		if id == "" {
			dest = "Inbox"
		}
		text := "Moved “" + t.Title + "” to " + dest
		undo := &undoEntry{desc: text, tasks: []domain.Task{before}}
		if created {
			undo.createdProjectID = id
			text += " · created project " + dest
		}
		return mutatedMsg{text: text, selectID: t.ID, undo: undo}, nil
	})
}

func deleteProject(s store.Store, p domain.Project, now time.Time) tea.Cmd {
	return mutate(func(ctx context.Context) (mutatedMsg, error) {
		tasks, err := s.ListTasks(ctx, domain.Filter{ProjectID: p.ID, IncludeDone: true, IncludeSubtasks: true},
			domain.SortManual)
		if err != nil {
			return mutatedMsg{}, err
		}
		if err := s.DeleteProject(ctx, p.ID, now); err != nil {
			return mutatedMsg{}, err
		}
		text := "Deleted project “" + p.Name + "”"
		return mutatedMsg{text: text + " · tasks moved to Inbox",
			undo: &undoEntry{desc: text, projects: []domain.Project{p}, tasks: tasks}}, nil
	})
}

// completeTask marks a task done by ID (from focus mode), undoably.
func completeTask(s store.Store, id string, now time.Time) tea.Cmd {
	return mutate(func(ctx context.Context) (mutatedMsg, error) {
		t, err := s.GetTask(ctx, id)
		if err != nil {
			return mutatedMsg{}, err
		}
		before := t
		t.SetStatus(domain.StatusDone, now)
		if err := s.UpdateTask(ctx, &t); err != nil {
			return mutatedMsg{}, err
		}
		text := "Completed “" + t.Title + "”"
		return mutatedMsg{text: text, selectID: t.ID, undo: &undoEntry{desc: text, tasks: []domain.Task{before}}}, nil
	})
}

type statsLoadedMsg struct {
	data stats.Data
	err  error
}

func loadStats(s store.Store, now time.Time) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := dbCtx()
		defer cancel()
		ts, err := s.Stats(ctx, now, stats.Days)
		if err != nil {
			return statsLoadedMsg{err: err}
		}
		from := domain.StartOfDay(now).AddDate(0, 0, -stats.Days+1)
		sessions, err := s.ListPomodoros(ctx, from, now.Add(time.Hour))
		if err != nil {
			return statsLoadedMsg{err: err}
		}
		return statsLoadedMsg{data: stats.Data{Tasks: ts, Focus: domain.ComputeFocus(sessions, stats.Days, now)}}
	}
}

// saveSetting writes one top-level config key, preserving the rest of the
// file. Failing to save a preference is reported but never fatal.
func saveSetting(path, key string, value any) tea.Cmd {
	if path == "" {
		return nil
	}
	return func() tea.Msg {
		if err := config.SetValue(path, key, value); err != nil {
			return common.StatusMsg{Text: "couldn't save config: " + err.Error(), Error: true}
		}
		return nil
	}
}

// markWelcomeSeen writes the commented default config on first run (it
// already records the welcome as seen), or flips the flag in an existing one.
func markWelcomeSeen(path string) tea.Cmd {
	if path == "" {
		return nil
	}
	return func() tea.Msg {
		if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
			if err := config.WriteTemplate(path); err != nil {
				return common.StatusMsg{Text: "couldn't write config: " + err.Error(), Error: true}
			}
			return nil
		}
		return saveSetting(path, "seen_welcome", true)()
	}
}

func seedSamples(s store.Store, now time.Time) tea.Cmd {
	return mutate(func(ctx context.Context) (mutatedMsg, error) {
		if err := service.SeedSamples(ctx, s, now); err != nil {
			return mutatedMsg{}, err
		}
		return mutatedMsg{text: "Added example tasks to “Getting started” — have a look around"}, nil
	})
}

// swapOrder exchanges the manual sort positions of two tasks.
func swapOrder(s store.Store, a, b domain.Task, now time.Time) tea.Cmd {
	return mutate(func(ctx context.Context) (mutatedMsg, error) {
		before := []domain.Task{a, b}
		a.SortOrder, b.SortOrder = b.SortOrder, a.SortOrder
		if a.SortOrder == b.SortOrder { // duplicates (e.g. after import): force an order
			b.SortOrder += 0.5
		}
		a.UpdatedAt, b.UpdatedAt = now, now
		if err := s.UpdateTask(ctx, &a); err != nil {
			return mutatedMsg{}, err
		}
		if err := s.UpdateTask(ctx, &b); err != nil {
			return mutatedMsg{}, err
		}
		return mutatedMsg{selectID: a.ID, undo: &undoEntry{desc: "Moved “" + a.Title + "”", tasks: before}}, nil
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
