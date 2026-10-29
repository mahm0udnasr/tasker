package tui

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/mahm0udnasr/tasker/internal/domain"
	"github.com/mahm0udnasr/tasker/internal/tui/common"
	"github.com/mahm0udnasr/tasker/internal/tui/form"
	"github.com/mahm0udnasr/tasker/internal/tui/overlay"
)

// action is a user command. The same registry backs key bindings (by name)
// and the command palette (by title), so the two can never drift apart.
type action struct {
	name      string // common.Act* — the binding and config key
	title     string // palette label
	needsTask bool   // requires a selected task
	run       func(m *Model, t domain.Task) tea.Cmd
}

// registry lists actions in palette order. Navigation (j/k, g/G, panes) is
// handled by the panes themselves and isn't listed here. It is filled in
// init because some actions (the palette) read the registry themselves.
var registry []action

func init() {
	registry = []action{
		{common.ActAdd, "Add task", false, func(m *Model, _ domain.Task) tea.Cmd { return m.openAddForm("", "") }},
		{common.ActAddSubtask, "Add subtask", true, (*Model).addSubtask},
		{common.ActEdit, "Edit task", true, (*Model).openEditForm},
		{common.ActToggleDone, "Toggle done", true, (*Model).toggleDone},
		{common.ActToggleStart, "Toggle in progress", true, (*Model).toggleStart},
		{common.ActPriority, "Cycle priority", true, (*Model).cyclePriority},
		{common.ActDue, "Set due date", true, (*Model).openDuePrompt},
		{common.ActMove, "Move to project", true, (*Model).openMovePicker},
		{common.ActDelete, "Delete task", true, (*Model).confirmDelete},
		{common.ActMoveUp, "Move task up", true, func(m *Model, t domain.Task) tea.Cmd { return m.reorder(t, -1) }},
		{common.ActMoveDown, "Move task down", true, func(m *Model, t domain.Task) tea.Cmd { return m.reorder(t, 1) }},
		{common.ActSearch, "Search all tasks", false, func(m *Model, _ domain.Task) tea.Cmd { return loadSearchIndex(m.store) }},
		{common.ActFilter, "Filter list", false, (*Model).openFilter},
		{common.ActClearFilter, "Clear filter", false, (*Model).clearFilter},
		{common.ActSort, "Cycle sort order", false, (*Model).cycleSort},
		{common.ActUndo, "Undo", false, (*Model).undoLast},
		{common.ActDetail, "Toggle detail pane", false, (*Model).toggleDetail},
		{common.ActDelProject, "Delete selected project", false, (*Model).confirmDeleteProject},
		{common.ActRefresh, "Reload", false, func(m *Model, _ domain.Task) tea.Cmd {
			return tea.Batch(m.reload(), common.Status("Reloaded"))
		}},
		{common.ActHelp, "Keyboard shortcuts", false, func(m *Model, _ domain.Task) tea.Cmd {
			return m.pushOverlay(overlay.NewHelp(m.ctx), nil)
		}},
		{common.ActPalette, "Command palette", false, (*Model).openPalette},
		{common.ActQuit, "Quit", false, func(*Model, domain.Task) tea.Cmd { return tea.Quit }},
	}
}

func findAction(name string) (action, bool) {
	for _, a := range registry {
		if a.name == name {
			return a, true
		}
	}
	return action{}, false
}

// keyAction returns the action bound to msg, if any. Task actions only fire
// when the list or detail pane has focus, so keys pressed while browsing the
// sidebar never edit a task by accident.
func (m *Model) keyAction(msg tea.KeyPressMsg) (action, bool) {
	for _, a := range registry {
		if !m.ctx.Keys.Is(msg, a.name) {
			continue
		}
		if a.needsTask && m.focus == paneSidebar {
			return action{}, false
		}
		return a, true
	}
	return action{}, false
}

// runAction executes a, resolving the selected task when needed.
func (m *Model) runAction(a action) tea.Cmd {
	t, ok := m.list.Selected()
	if a.needsTask && !ok {
		return common.ErrorCmd(errors.New("no task selected"))
	}
	return a.run(m, t)
}

// --- task actions -------------------------------------------------------------

func (m *Model) addSubtask(t domain.Task) tea.Cmd {
	parent := t
	if t.IsSubtask() {
		if p, ok := m.list.Find(t.ParentID); ok {
			parent = p
		}
	}
	return m.openAddForm(parent.ID, parent.Title)
}

func (m *Model) toggleDone(t domain.Task) tea.Cmd {
	t.ToggleDone(m.ctx.Now())
	text := "Completed “" + t.Title + "”"
	if !t.IsDone() {
		text = "Reopened “" + t.Title + "”"
	}
	return updateTask(m.store, t, text)
}

func (m *Model) toggleStart(t domain.Task) tea.Cmd {
	next := domain.StatusInProgress
	if t.Status == domain.StatusInProgress {
		next = domain.StatusTodo
	}
	t.SetStatus(next, m.ctx.Now())
	return updateTask(m.store, t, "“"+t.Title+"” is "+next.Label())
}

func (m *Model) cyclePriority(t domain.Task) tea.Cmd {
	t.CyclePriority(m.ctx.Now())
	return updateTask(m.store, t, "Priority of “"+t.Title+"”: "+t.Priority.String())
}

func (m *Model) confirmDelete(t domain.Task) tea.Cmd {
	body := fmt.Sprintf("“%s” will be deleted.", t.Title)
	if n := t.SubtasksTotal; n > 0 {
		body = fmt.Sprintf("“%s” and its %s will be deleted.", t.Title, plural(n, "subtask"))
	}
	body += " You can undo with " + m.ctx.Keys.Get(common.ActUndo).Help().Key + "."
	return m.pushOverlay(overlay.NewConfirm(m.ctx, "Delete task?", body,
		taskActionMsg{kind: actDelete, task: t}), nil)
}

// reorder swaps t with its sibling in manual order.
func (m *Model) reorder(t domain.Task, delta int) tea.Cmd {
	if m.sortKey() != domain.SortManual {
		return common.ErrorCmd(fmt.Errorf("reordering needs manual sort — press %s to change the sort",
			m.ctx.Keys.Get(common.ActSort).Help().Key))
	}
	other, ok := m.list.Neighbor(delta)
	if !ok {
		return nil
	}
	return swapOrder(m.store, t, other, m.ctx.Now())
}

// --- list-wide actions ---------------------------------------------------------

func (m *Model) openFilter(domain.Task) tea.Cmd {
	return m.pushOverlay(overlay.NewFilter(m.ctx, m.refine), nil)
}

func (m *Model) setRefinement(r domain.Refinement) tea.Cmd {
	m.refine = r
	m.updateListTitle()
	m.list.ResetCursor()
	text := "Filter cleared"
	if r.Active() {
		text = "Filtered: " + r.Describe()
	}
	return tea.Batch(m.reloadTasks(), common.Status(text))
}

func (m *Model) clearFilter(domain.Task) tea.Cmd {
	if !m.refine.Active() {
		return nil
	}
	return m.setRefinement(domain.Refinement{})
}

func (m *Model) cycleSort(domain.Task) tea.Cmd {
	next := m.sortKey().Next()
	if next == domain.DefaultSort(m.scope.View) {
		delete(m.sorts, m.scope.Key())
	} else {
		m.sorts[m.scope.Key()] = next
	}
	m.updateListTitle()
	return tea.Batch(m.reloadTasks(), common.Status("Sorted by "+string(next)))
}

func (m *Model) undoLast(domain.Task) tea.Cmd {
	e, ok := m.undo.pop()
	if !ok {
		return common.Status("Nothing to undo")
	}
	return applyUndo(m.store, e, m.ctx.Now())
}

func (m *Model) toggleDetail(domain.Task) tea.Cmd {
	if m.canShowDetailPane() {
		m.showDetail = !m.showDetail
		if !m.showDetail && m.focus == paneDetail {
			m.focus = paneList
		}
		m.layout()
		m.applyFocus()
		return m.syncDetail(true)
	}
	// Too narrow for a third pane: show the details as a modal instead.
	if _, ok := m.list.Selected(); !ok {
		return common.ErrorCmd(errors.New("no task selected"))
	}
	return m.pushOverlay(newDetailOverlay(m.ctx, m.detail), nil)
}

// --- palette & search ------------------------------------------------------------

// paletteMsg runs the named action chosen in the palette.
type paletteMsg struct{ name string }

// jumpScopeMsg selects a sidebar scope (palette "Go to …").
type jumpScopeMsg struct{ scope common.Scope }

// jumpToTaskMsg opens the view containing a task and selects it.
type jumpToTaskMsg struct {
	task   domain.Task
	parent *domain.Task
}

func (m *Model) openPalette(domain.Task) tea.Cmd {
	ic := m.ctx.Icons
	var items []overlay.PickerItem
	for _, a := range registry {
		if a.name == common.ActPalette {
			continue
		}
		items = append(items, overlay.PickerItem{Icon: "›", Label: a.title,
			Hint: m.ctx.Keys.Get(a.name).Help().Key, Value: paletteMsg{name: a.name}})
	}
	viewIcons := map[domain.SmartView]string{domain.ViewInbox: ic.Inbox, domain.ViewToday: ic.Today,
		domain.ViewUpcoming: ic.Upcoming, domain.ViewOverdue: ic.Overdue, domain.ViewCompleted: ic.Completed}
	for _, v := range domain.SmartViews {
		items = append(items, overlay.PickerItem{Icon: viewIcons[v], Label: "Go to " + v.Title(),
			Hint: fmt.Sprint(m.counts.For(v)), Value: jumpScopeMsg{common.Scope{View: v, Title: v.Title()}}})
	}
	for _, p := range m.projects {
		items = append(items, overlay.PickerItem{Icon: ic.Project, Label: "Go to project " + p.Name,
			Hint:  fmt.Sprint(m.counts.Projects[p.ID]),
			Value: jumpScopeMsg{common.Scope{View: domain.ViewAll, ProjectID: p.ID, Title: p.Name}}})
	}
	for _, tag := range sortedKeys(m.counts.Tags) {
		items = append(items, overlay.PickerItem{Icon: ic.Tag, Label: "Go to tag #" + tag,
			Hint:  fmt.Sprint(m.counts.Tags[tag]),
			Value: jumpScopeMsg{common.Scope{View: domain.ViewAll, Tag: tag, Title: "#" + tag}}})
	}
	p, init := overlay.NewPicker(m.ctx, overlay.PickerOptions{
		Title:       "Command palette",
		Placeholder: "type a command…",
		Items:       items,
		Width:       56,
		OnPick:      func(it overlay.PickerItem) tea.Msg { return it.Value },
	})
	return m.pushOverlay(p, init)
}

func (m *Model) openSearch(tasks []domain.Task) tea.Cmd {
	byID := make(map[string]domain.Task, len(tasks))
	for _, t := range tasks {
		byID[t.ID] = t
	}
	now := m.ctx.Now()
	items := make([]overlay.PickerItem, 0, len(tasks))
	for _, t := range tasks {
		icon := m.ctx.Icons.Todo
		switch t.Status {
		case domain.StatusDone:
			icon = m.ctx.Icons.Done
		case domain.StatusInProgress:
			icon = m.ctx.Icons.InProgress
		}
		var hint []string
		var parent *domain.Task
		if p, ok := byID[t.ParentID]; ok {
			parent = &p
			hint = append(hint, m.ctx.Icons.Subtasks+" "+truncate(p.Title, 14))
		} else if name := m.projectName(t.ProjectID); name != "" {
			hint = append(hint, name)
		}
		if t.Due != nil && !t.IsDone() {
			hint = append(hint, domain.FormatDue(*t.Due, t.DueHasTime, now))
		}
		search := t.Title + " " + strings.Join(t.Tags, " ") + " " + m.projectName(t.ProjectID)
		items = append(items, overlay.PickerItem{Icon: icon, Label: t.Title, Search: search,
			Hint: strings.Join(hint, " · "), Value: jumpToTaskMsg{task: t, parent: parent}})
	}
	p, init := overlay.NewPicker(m.ctx, overlay.PickerOptions{
		Title:       "Search " + plural(len(tasks), "task"),
		Placeholder: "title, tag or project…",
		Items:       items,
		Width:       64,
		MaxRows:     12,
		OnPick:      func(it overlay.PickerItem) tea.Msg { return it.Value },
	})
	return m.pushOverlay(p, init)
}

// jumpToTask switches to the view that shows the task and selects it:
// Completed if done, else its project or the Inbox; subtasks via their
// (expanded) parent.
func (m *Model) jumpToTask(msg jumpToTaskMsg) tea.Cmd {
	target := msg.task
	if msg.parent != nil {
		target = *msg.parent
		m.list.Expand(target.ID)
	}
	var scope common.Scope
	switch {
	case target.IsDone():
		scope = common.Scope{View: domain.ViewCompleted, Title: domain.ViewCompleted.Title()}
	case target.ProjectID != "":
		scope = common.Scope{View: domain.ViewAll, ProjectID: target.ProjectID, Title: m.projectName(target.ProjectID)}
	default:
		scope = common.Scope{View: domain.ViewInbox, Title: domain.ViewInbox.Title()}
	}
	m.refine = domain.Refinement{} // a filter could hide the target
	m.pendingSelect = msg.task.ID
	m.focus = paneList
	return m.jumpScope(scope)
}

func (m *Model) jumpScope(s common.Scope) tea.Cmd {
	m.sidebar.Select(s)
	m.setScope(s)
	m.list.ResetCursor()
	m.applyFocus()
	return m.reloadTasks()
}

// --- overlays ------------------------------------------------------------------

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

func (m *Model) finishTaskAction(a taskActionMsg) tea.Cmd {
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
		text := "Cleared due date of “" + t.Title + "”"
		if due != nil {
			text = "“" + t.Title + "” due " + domain.FormatDue(*due, hasTime, now)
		}
		return updateTask(m.store, t, text)
	case actMove:
		return moveTask(m.store, a.task, a.value, now)
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
			return "→ " + due.Format(layout) + " (" + domain.Relative(*due, hasTime, m.ctx.Now()) + ")", true
		},
		OnSubmit: func(s string) tea.Msg { return taskActionMsg{kind: actSetDue, task: t, value: s} },
	})
	return m.pushOverlay(p, init)
}

// inboxValue marks the Inbox entry in the move picker.
const inboxValue = "\x00inbox"

func (m *Model) openMovePicker(t domain.Task) tea.Cmd {
	if t.IsSubtask() {
		return common.ErrorCmd(errors.New("subtasks move with their parent"))
	}
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
func (m *Model) confirmDeleteProject(domain.Task) tea.Cmd {
	if m.scope.ProjectID == "" {
		return common.ErrorCmd(errors.New("select a project in the sidebar to delete it"))
	}
	p := m.project(m.scope.ProjectID)
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

func sortedKeys(m map[string]int) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}
