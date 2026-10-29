// Package tasklist renders the main task list: priority markers, status,
// due badges, tag chips and subtask progress, with expandable subtasks.
package tasklist

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/mahm0udnasr/tasker/internal/domain"
	"github.com/mahm0udnasr/tasker/internal/tui/common"
)

// Row is one visible line: a task, or a subtask under an expanded parent.
type Row struct {
	Task  domain.Task
	Depth int  // 0 = top-level, 1 = subtask
	Last  bool // last subtask of its parent (draws └ instead of ├)
}

// Model is the task list.
type Model struct {
	ctx      *common.Context
	rows     []Row
	expanded map[string]bool
	cursor   int
	offset   int
	width    int
	height   int
	focused  bool
	title    string
	badges   []string // sort/filter indicators appended to the title
	empty    string   // message when there are no rows
	loaded   bool
}

// New creates an empty list.
func New(ctx *common.Context) Model {
	return Model{ctx: ctx, expanded: map[string]bool{}}
}

// SetRows replaces the rows, keeping the cursor on the same task if it is
// still present (so a reload after toggling doesn't make the cursor jump).
func (m *Model) SetRows(rows []Row) {
	prevID := ""
	if t, ok := m.Selected(); ok {
		prevID = t.ID
	}
	m.rows = rows
	m.loaded = true
	if prevID != "" {
		for i, r := range rows {
			if r.Task.ID == prevID {
				m.cursor = i
				m.clamp()
				return
			}
		}
	}
	m.clamp()
}

// SetTitle sets the panel title and the empty-state message.
func (m *Model) SetTitle(title, empty string) { m.title, m.empty = title, empty }

// SetBadges sets short indicators shown after the title and count.
func (m *Model) SetBadges(b []string) { m.badges = b }

// Neighbor returns the adjacent sibling of the selected task in direction
// delta (±1): the next top-level task for a top-level row (skipping
// expanded subtasks), or the next subtask of the same parent.
func (m Model) Neighbor(delta int) (domain.Task, bool) {
	cur, ok := m.Selected()
	if !ok {
		return domain.Task{}, false
	}
	for i := m.cursor + delta; i >= 0 && i < len(m.rows); i += delta {
		r := m.rows[i]
		if r.Task.ParentID == cur.ParentID {
			return r.Task, true
		}
		if cur.ParentID != "" { // left the parent's block
			break
		}
	}
	return domain.Task{}, false
}

// ResetCursor moves to the top (used when the scope changes).
func (m *Model) ResetCursor() { m.cursor, m.offset = 0, 0 }

// SelectID moves the cursor to the task with id, if visible.
func (m *Model) SelectID(id string) {
	for i, r := range m.rows {
		if r.Task.ID == id {
			m.cursor = i
			m.clamp()
			return
		}
	}
}

// Find returns a visible task by ID.
func (m Model) Find(id string) (domain.Task, bool) {
	for _, r := range m.rows {
		if r.Task.ID == id {
			return r.Task, true
		}
	}
	return domain.Task{}, false
}

// Expanded returns the IDs of parents whose subtasks are shown.
func (m Model) Expanded() map[string]bool { return m.expanded }

// Expand shows a parent's subtasks.
func (m *Model) Expand(id string) { m.expanded[id] = true }

// Selected returns the task under the cursor.
func (m Model) Selected() (domain.Task, bool) {
	if m.cursor < 0 || m.cursor >= len(m.rows) {
		return domain.Task{}, false
	}
	return m.rows[m.cursor].Task, true
}

// Len is the number of visible rows.
func (m Model) Len() int { return len(m.rows) }

// SetSize sets the outer size including the border.
func (m *Model) SetSize(w, h int) {
	m.width, m.height = w, h
	m.clamp()
}

// SetFocused toggles focus styling.
func (m *Model) SetFocused(f bool) { m.focused = f }

func (m Model) visible() int { return max(m.height-2, 1) }

func (m *Model) clamp() {
	m.cursor = max(0, min(m.cursor, len(m.rows)-1))
	v := m.visible()
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+v {
		m.offset = m.cursor - v + 1
	}
	m.offset = max(0, min(m.offset, len(m.rows)-v))
}

// ExpandToggledMsg asks the root to reload so subtasks appear/disappear.
type ExpandToggledMsg struct{}

// Update handles navigation and expand/collapse.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	km, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	keys := m.ctx.Keys
	switch {
	case keys.Is(km, common.ActUp):
		m.cursor--
	case keys.Is(km, common.ActDown):
		m.cursor++
	case keys.Is(km, common.ActTop):
		m.cursor = 0
	case keys.Is(km, common.ActBottom):
		m.cursor = len(m.rows) - 1
	case keys.Is(km, common.ActPageUp):
		m.cursor -= m.visible() / 2
	case keys.Is(km, common.ActPageDown):
		m.cursor += m.visible() / 2
	case keys.Is(km, common.ActExpand):
		return m.toggleExpand()
	default:
		return m, nil
	}
	m.clamp()
	return m, nil
}

func (m Model) toggleExpand() (Model, tea.Cmd) {
	t, ok := m.Selected()
	if !ok {
		return m, nil
	}
	switch {
	case t.IsSubtask(): // collapse the parent from one of its children
		delete(m.expanded, t.ParentID)
		m.SelectID(t.ParentID)
	case t.SubtasksTotal == 0:
		return m, nil
	case m.expanded[t.ID]:
		delete(m.expanded, t.ID)
	default:
		m.expanded[t.ID] = true
	}
	return m, func() tea.Msg { return ExpandToggledMsg{} }
}

// View renders the panel.
func (m Model) View() string {
	st := m.ctx.Styles
	inner := m.width - 2
	title := m.title
	if m.loaded {
		top := 0
		for _, r := range m.rows {
			if r.Depth == 0 {
				top++
			}
		}
		title = fmt.Sprintf("%s · %d", m.title, top)
	}
	for _, b := range m.badges {
		title += " · " + b
	}

	var body string
	if m.loaded && len(m.rows) == 0 {
		body = m.emptyView(inner, m.height-2)
	} else {
		lines := make([]string, 0, m.visible())
		for i := m.offset; i < len(m.rows) && len(lines) < m.visible(); i++ {
			lines = append(lines, m.renderRow(m.rows[i], i == m.cursor, inner))
		}
		body = strings.Join(lines, "\n")
	}
	return st.Panel(title, body, m.width, m.height, m.focused)
}

func (m Model) emptyView(w, h int) string {
	st := m.ctx.Styles
	msg := lipgloss.JoinVertical(lipgloss.Center,
		st.Accent.Render(m.ctx.Icons.Completed),
		"",
		st.Empty.Render(m.empty),
		"",
		st.KeyDesc.Render("press ")+st.KeyHint.Render(m.ctx.Keys.Get(common.ActAdd).Help().Key)+
			st.KeyDesc.Render(" to add a task"),
	)
	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, msg)
}

// renderRow lays out one row in exactly w cells:
//
//	▌▎ ▸ ○ Title……………… 2/5  #tag #tag   tomorrow 17:00
//
// Every segment is rendered with the row background when selected, because
// an inner ANSI reset would otherwise cut the highlight short.
func (m Model) renderRow(r Row, selected bool, w int) string {
	st := m.ctx.Styles
	ic := m.ctx.Icons
	th := st.Theme
	t := r.Task
	now := m.ctx.Now()

	paint := func(s lipgloss.Style) lipgloss.Style {
		if selected {
			return s.Background(th.Surface)
		}
		return s
	}
	plain := paint(lipgloss.NewStyle())

	// Left gutter: cursor marker + priority bar.
	cursor := " "
	if selected && m.focused {
		cursor = st.Accent.Render(ic.Selected)
	}
	prio := paint(lipgloss.NewStyle().Foreground(th.Priority[clampPrio(t.Priority)])).Render(ic.Priority)
	if t.Priority == domain.PriorityNone {
		prio = plain.Render(strings.Repeat(" ", lipgloss.Width(ic.Priority)))
	}

	// Tree / expander.
	var tree string
	switch {
	case r.Depth > 0 && r.Last:
		tree = paint(st.Muted).Render("  " + ic.BranchLast + " ")
	case r.Depth > 0:
		tree = paint(st.Muted).Render("  " + ic.Branch + " ")
	case t.SubtasksTotal > 0 && m.expanded[t.ID]:
		tree = paint(st.Muted).Render(" " + ic.Expanded + " ")
	case t.SubtasksTotal > 0:
		tree = paint(st.Muted).Render(" " + ic.Collapsed + " ")
	default:
		tree = plain.Render("   ")
	}

	status := statusIcon(m.ctx, t.Status, paint)

	// Right side, built first so the title gets whatever is left. Each
	// segment has a drop rank: when the title would get too narrow, the
	// lowest-ranked segments are dropped first (due date survives longest).
	type segment struct {
		text string
		rank int
	}
	var segs []segment
	if p := t.SubtaskProgress(); p != "" {
		s := paint(st.Muted)
		if t.SubtasksDone == t.SubtasksTotal {
			s = paint(st.Success)
		}
		segs = append(segs, segment{s.Render(ic.Subtasks + " " + p), 3})
	}
	if t.Description != "" {
		segs = append(segs, segment{paint(st.Muted).Render(ic.Note), 2})
	}
	for i, tag := range t.Tags {
		if i == 3 {
			segs = append(segs, segment{paint(st.Muted).Render(fmt.Sprintf("+%d", len(t.Tags)-3)), 0})
			break
		}
		rank := 1
		if i > 0 {
			rank = 0
		}
		segs = append(segs, segment{paint(st.Tag).Render("#" + tag), rank})
	}
	if t.Due != nil {
		ds := paint(st.DueLater)
		switch {
		case t.IsDone():
			ds = paint(st.Muted)
		case t.IsOverdue(now):
			ds = paint(st.DueOverdue)
		case t.IsDueToday(now):
			ds = paint(st.DueToday)
		}
		segs = append(segs, segment{ds.Render(ic.Due + " " + domain.FormatDue(*t.Due, t.DueHasTime, now)), 4})
	}

	left := cursor + prio + tree + status + plain.Render(" ")
	minTitle := min(lipgloss.Width(t.Title), 24)
	var rightStr string
	for rank := 0; ; rank++ {
		parts := make([]string, 0, len(segs))
		for _, sg := range segs {
			if sg.rank >= rank {
				parts = append(parts, sg.text)
			}
		}
		rightStr = strings.Join(parts, plain.Render("  "))
		if w-lipgloss.Width(left)-lipgloss.Width(rightStr)-2 >= minTitle || rank > 4 {
			break
		}
	}
	titleW := w - lipgloss.Width(left) - lipgloss.Width(rightStr) - 2
	if titleW < 8 { // still too narrow: drop the right side entirely
		rightStr = ""
		titleW = w - lipgloss.Width(left) - 1
	}
	ts := paint(st.Row)
	if t.IsDone() {
		ts = paint(st.DoneTitle)
	} else if selected {
		ts = ts.Bold(true)
	}
	title := truncate(t.Title, titleW)
	gap := max(w-lipgloss.Width(left)-lipgloss.Width(title)-lipgloss.Width(rightStr)-1, 1)
	return left + ts.Render(title) + plain.Render(strings.Repeat(" ", gap)) + rightStr + plain.Render(" ")
}

func statusIcon(ctx *common.Context, s domain.Status, paint func(lipgloss.Style) lipgloss.Style) string {
	st := ctx.Styles
	switch s {
	case domain.StatusDone:
		return paint(st.Success).Render(ctx.Icons.Done)
	case domain.StatusInProgress:
		return paint(st.Warning).Render(ctx.Icons.InProgress)
	default:
		return paint(st.Muted).Render(ctx.Icons.Todo)
	}
}

func clampPrio(p domain.Priority) int {
	return max(0, min(int(p), 4))
}

func truncate(s string, w int) string {
	if w <= 1 {
		return ""
	}
	if lipgloss.Width(s) <= w {
		return s
	}
	r := []rune(s)
	for len(r) > 0 && lipgloss.Width(string(r))+1 > w {
		r = r[:len(r)-1]
	}
	return string(r) + "…"
}
