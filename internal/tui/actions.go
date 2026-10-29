package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/mahm0udnasr/tasker/internal/domain"
	"github.com/mahm0udnasr/tasker/internal/tui/common"
	"github.com/mahm0udnasr/tasker/internal/tui/form"
	"github.com/mahm0udnasr/tasker/internal/tui/overlay"
)

type actionKind int

const (
	actDelete actionKind = iota
	actSetDue
	actMove
)

// taskActionMsg is produced by an overlay (confirm, prompt, picker) once the
// user has supplied what an action needs.
type taskActionMsg struct {
	kind  actionKind
	task  domain.Task
	value string
}

func (m *Model) runAction(a taskActionMsg) tea.Cmd {
	now := m.ctx.Now()
	switch a.kind {
	case actDelete:
		return deleteTask(m.store, a.task, now)
	case actSetDue:
		due, hasTime, err := domain.ParseDue(a.value, now)
		if err != nil {
			return common.ErrorCmd(err)
		}
		t := a.task
		t.SetDue(due, hasTime, now)
		text := "Cleared due date"
		if due != nil {
			text = "Due " + domain.FormatDue(*due, hasTime, now)
		}
		return updateTask(m.store, t, text)
	case actMove:
		return moveTask(m.store, a.task, a.value, now)
	}
	return nil
}

// taskKey handles actions on the selected task. It returns nil when the key
// is not a task action, so the list can treat it as navigation.
func (m *Model) taskKey(msg tea.KeyPressMsg) tea.Cmd {
	k := m.ctx.Keys
	t, ok := m.list.Selected()
	now := m.ctx.Now()
	if !ok {
		return nil
	}
	switch {
	case k.Is(msg, common.ActAddSubtask):
		parent := t
		if t.IsSubtask() {
			if p, ok := m.list.Find(t.ParentID); ok {
				parent = p
			}
		}
		return m.openAddForm(parent.ID, parent.Title)

	case k.Is(msg, common.ActEdit):
		return m.openEditForm(t)

	case k.Is(msg, common.ActToggleDone):
		t.ToggleDone(now)
		text := "Completed “" + t.Title + "”"
		if !t.IsDone() {
			text = "Reopened “" + t.Title + "”"
		}
		return updateTask(m.store, t, text)

	case k.Is(msg, common.ActToggleStart):
		next := domain.StatusInProgress
		if t.Status == domain.StatusInProgress {
			next = domain.StatusTodo
		}
		t.SetStatus(next, now)
		return updateTask(m.store, t, "Status: "+next.Label())

	case k.Is(msg, common.ActPriority):
		t.CyclePriority(now)
		return updateTask(m.store, t, "Priority: "+t.Priority.String())

	case k.Is(msg, common.ActDelete):
		body := fmt.Sprintf("“%s” will be deleted.", t.Title)
		if n := t.SubtasksTotal; n > 0 {
			body = fmt.Sprintf("“%s” and its %s will be deleted.", t.Title, plural(n, "subtask"))
		}
		return m.pushOverlay(overlay.NewConfirm(m.ctx, "Delete task?", body,
			taskActionMsg{kind: actDelete, task: t}), nil)

	case k.Is(msg, common.ActDue):
		return m.openDuePrompt(t)

	case k.Is(msg, common.ActMove):
		if t.IsSubtask() {
			return common.ErrorCmd(fmt.Errorf("subtasks move with their parent"))
		}
		return m.openMovePicker(t)
	}
	return nil
}

func (m *Model) openAddForm(parentID, parentName string) tea.Cmd {
	o := form.Options{ParentID: parentID, ParentName: parentName, Projects: m.projectNames()}
	// Pre-fill from the current scope so a task added in a view shows up there.
	o.Project = m.projectName(m.scope.ProjectID)
	if parentID == "" {
		switch m.scope.View {
		case domain.ViewToday:
			o.Due = "today"
		case domain.ViewUpcoming:
			o.Due = "tomorrow"
		}
		if m.scope.Tag != "" {
			o.Tags = m.scope.Tag
		}
	}
	f, init := form.New(m.ctx, o)
	return m.pushOverlay(f, init)
}

func (m *Model) openEditForm(t domain.Task) tea.Cmd {
	f, init := form.New(m.ctx, form.Options{
		Task: &t, Project: m.projectName(t.ProjectID), Projects: m.projectNames(),
	})
	return m.pushOverlay(f, init)
}

func (m *Model) openDuePrompt(t domain.Task) tea.Cmd {
	value := ""
	if t.Due != nil {
		value = t.Due.Format("2006-01-02")
		if t.DueHasTime {
			value = t.Due.Format("2006-01-02 15:04")
		}
	}
	p, init := overlay.NewPrompt(m.ctx, overlay.PromptOptions{
		Title:       "Due date · " + t.Title,
		Value:       value,
		Placeholder: `"tomorrow 5pm", "next fri", "in 3 days"`,
		Suggestions: []string{"today", "tonight", "tomorrow", "tomorrow 9am", "next week", "weekend", "next month", "none"},
		Help:        "tab completes · empty or “none” clears · enter save · esc cancel",
		Preview: func(s string) (string, bool) {
			due, hasTime, err := domain.ParseDue(s, m.ctx.Now())
			switch {
			case err != nil:
				return m.ctx.Icons.Cross + " can't read that date", false
			case due == nil:
				return "no due date", true
			}
			layout := "Monday 2 January 2006"
			if hasTime {
				layout += " · 15:04"
			}
			return "→ " + due.Format(layout), true
		},
		OnSubmit: func(s string) tea.Msg { return taskActionMsg{kind: actSetDue, task: t, value: s} },
	})
	return m.pushOverlay(p, init)
}

// inboxValue marks the Inbox entry in the move picker.
const inboxValue = "\x00inbox"

func (m *Model) openMovePicker(t domain.Task) tea.Cmd {
	items := []overlay.PickerItem{{Icon: m.ctx.Icons.Inbox, Label: "Inbox", Hint: "no project", Value: inboxValue}}
	for _, p := range m.projects {
		icon := p.Icon
		if icon == "" {
			icon = m.ctx.Icons.Project
		}
		items = append(items, overlay.PickerItem{Icon: icon, Label: p.Name,
			Hint: fmt.Sprintf("%d open", m.counts.Projects[p.ID]), Value: p.ID})
	}
	current := m.projectName(t.ProjectID)
	if current == "" {
		current = "Inbox"
	}
	p, init := overlay.NewPicker(m.ctx, overlay.PickerOptions{
		Title:        "Move “" + t.Title + "” to…",
		Placeholder:  "type to filter or name a new project",
		Items:        items,
		Current:      current,
		CreateFormat: "Create project “%s”",
		OnPick: func(it overlay.PickerItem) tea.Msg {
			name := it.Label // an existing project, or a new name to create
			if it.Value == inboxValue {
				name = ""
			}
			return taskActionMsg{kind: actMove, task: t, value: name}
		},
	})
	return m.pushOverlay(p, init)
}

// deleteProjectMsg is delivered by the confirm dialog.
type deleteProjectMsg struct{ project domain.Project }

// confirmDeleteProject asks before deleting the project selected in the
// sidebar. Its tasks are kept and move to the Inbox.
func (m *Model) confirmDeleteProject() tea.Cmd {
	if m.scope.ProjectID == "" {
		return common.ErrorCmd(fmt.Errorf("select a project in the sidebar to delete it"))
	}
	var p domain.Project
	for _, x := range m.projects {
		if x.ID == m.scope.ProjectID {
			p = x
		}
	}
	if p.ID == "" {
		return nil
	}
	body := fmt.Sprintf("Project “%s” will be deleted. It has no open tasks.", p.Name)
	if n := m.counts.Projects[p.ID]; n > 0 {
		body = fmt.Sprintf("Project “%s” will be deleted. Its %s will move to the Inbox.",
			p.Name, plural(n, "open task"))
	}
	return m.pushOverlay(overlay.NewConfirm(m.ctx, "Delete project?", body, deleteProjectMsg{project: p}), nil)
}

func (m *Model) openHelp() tea.Cmd {
	return m.pushOverlay(overlay.NewHelp(m.ctx), nil)
}
