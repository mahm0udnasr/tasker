// Package sidebar renders the smart views, projects and tags with live counts
// and reports the selected Scope.
package sidebar

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/mahm0udnasr/tasker/internal/domain"
	"github.com/mahm0udnasr/tasker/internal/tui/common"
)

type itemKind int

const (
	kindView itemKind = iota
	kindProject
	kindTag
	kindHeader // not selectable
	kindSpacer // not selectable
)

type item struct {
	kind  itemKind
	scope common.Scope
	icon  string
	color lipgloss.Style
	count int
}

func (i item) selectable() bool { return i.kind != kindHeader && i.kind != kindSpacer }

// Model is the sidebar.
type Model struct {
	ctx      *common.Context
	items    []item
	cursor   int
	offset   int
	width    int
	height   int
	focused  bool
	projects []domain.Project
	tags     []domain.Tag
	counts   domain.Counts
}

// New creates a sidebar that starts on the given view.
func New(ctx *common.Context, start domain.SmartView) Model {
	m := Model{ctx: ctx}
	m.rebuild()
	for i, it := range m.items {
		if it.kind == kindView && it.scope.View == start {
			m.cursor = i
		}
	}
	return m
}

// SetData refreshes projects, tags and counts, keeping the selection on the
// same scope when it still exists.
func (m *Model) SetData(projects []domain.Project, tags []domain.Tag, counts domain.Counts) {
	prev := m.Selected().Key()
	m.projects, m.tags, m.counts = projects, tags, counts
	m.rebuild()
	m.cursor = m.firstSelectable()
	for i, it := range m.items {
		if it.selectable() && it.scope.Key() == prev {
			m.cursor = i
			break
		}
	}
	m.clampOffset()
}

func (m *Model) rebuild() {
	ic := m.ctx.Icons
	st := m.ctx.Styles
	viewIcons := map[domain.SmartView]string{
		domain.ViewInbox: ic.Inbox, domain.ViewToday: ic.Today, domain.ViewUpcoming: ic.Upcoming,
		domain.ViewOverdue: ic.Overdue, domain.ViewCompleted: ic.Completed,
	}
	viewColors := map[domain.SmartView]lipgloss.Style{
		domain.ViewInbox: st.Accent, domain.ViewToday: st.Warning, domain.ViewUpcoming: st.Project,
		domain.ViewOverdue: st.Danger, domain.ViewCompleted: st.Success,
	}
	items := make([]item, 0, len(domain.SmartViews)+len(m.projects)+len(m.tags)+4)
	for _, v := range domain.SmartViews {
		items = append(items, item{kind: kindView, scope: common.Scope{View: v, Title: v.Title()},
			icon: viewIcons[v], color: viewColors[v], count: m.counts.For(v)})
	}
	items = append(items, item{kind: kindSpacer}, item{kind: kindHeader, scope: common.Scope{Title: "PROJECTS"}})
	if len(m.projects) == 0 {
		items = append(items, item{kind: kindHeader, scope: common.Scope{Title: "  none yet"}})
	}
	for _, p := range m.projects {
		icon := p.Icon
		if icon == "" {
			icon = ic.Project
		}
		color := st.Project
		if p.Color != "" {
			color = lipgloss.NewStyle().Foreground(lipgloss.Color(p.Color))
		}
		items = append(items, item{kind: kindProject, icon: icon, color: color, count: m.counts.Projects[p.ID],
			scope: common.Scope{View: domain.ViewAll, ProjectID: p.ID, Title: p.Name}})
	}
	if len(m.tags) > 0 {
		items = append(items, item{kind: kindSpacer}, item{kind: kindHeader, scope: common.Scope{Title: "TAGS"}})
		for _, t := range m.tags {
			items = append(items, item{kind: kindTag, icon: ic.Tag, color: st.Tag, count: t.Count,
				scope: common.Scope{View: domain.ViewAll, Tag: t.Name, Title: "#" + t.Name}})
		}
	}
	m.items = items
}

func (m Model) firstSelectable() int {
	for i, it := range m.items {
		if it.selectable() {
			return i
		}
	}
	return 0
}

// Selected returns the highlighted scope.
func (m Model) Selected() common.Scope {
	if m.cursor < 0 || m.cursor >= len(m.items) {
		return common.Scope{View: domain.ViewInbox, Title: "Inbox"}
	}
	return m.items[m.cursor].scope
}

// SetSize sets the outer size including the border.
func (m *Model) SetSize(w, h int) {
	m.width, m.height = w, h
	m.clampOffset()
}

// SetFocused toggles focus styling.
func (m *Model) SetFocused(f bool) { m.focused = f }

// Update handles navigation while the sidebar is focused. Moving the cursor
// immediately selects the new scope, lazygit-style.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	km, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	keys := m.ctx.Keys
	prev := m.cursor
	switch {
	case keys.Is(km, common.ActUp):
		m.move(-1)
	case keys.Is(km, common.ActDown):
		m.move(1)
	case keys.Is(km, common.ActTop):
		m.cursor = m.firstSelectable()
	case keys.Is(km, common.ActBottom):
		for i := len(m.items) - 1; i >= 0; i-- {
			if m.items[i].selectable() {
				m.cursor = i
				break
			}
		}
	default:
		return m, nil
	}
	m.clampOffset()
	if m.cursor == prev {
		return m, nil
	}
	scope := m.Selected()
	return m, func() tea.Msg { return common.ScopeSelectedMsg{Scope: scope} }
}

func (m *Model) move(delta int) {
	for i := m.cursor + delta; i >= 0 && i < len(m.items); i += delta {
		if m.items[i].selectable() {
			m.cursor = i
			return
		}
	}
}

func (m *Model) clampOffset() {
	visible := m.height - 2
	if visible <= 0 {
		return
	}
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+visible {
		m.offset = m.cursor - visible + 1
	}
	m.offset = max(0, min(m.offset, len(m.items)-visible))
}

// View renders the panel.
func (m Model) View() string {
	st := m.ctx.Styles
	inner := m.width - 2
	visible := m.height - 2
	var lines []string
	for i := m.offset; i < len(m.items) && len(lines) < visible; i++ {
		lines = append(lines, m.renderItem(m.items[i], i == m.cursor, inner))
	}
	return st.Panel("tasker", strings.Join(lines, "\n"), m.width, m.height, m.focused)
}

func (m Model) renderItem(it item, selected bool, w int) string {
	st := m.ctx.Styles
	switch it.kind {
	case kindSpacer:
		return ""
	case kindHeader:
		return st.SidebarHeader.Render(" " + it.scope.Title)
	}

	count := ""
	if it.count > 0 {
		count = fmt.Sprintf("%d", it.count)
	}
	marker := " "
	if selected && m.focused {
		marker = st.Accent.Render(m.ctx.Icons.Selected)
	}

	// Layout: marker ␣ icon ␣ name … count ␣  (exactly w cells)
	iconW := lipgloss.Width(it.icon)
	name := truncate(it.scope.Title, w-iconW-len(count)-5)
	gap := max(w-1-1-iconW-1-lipgloss.Width(name)-len(count)-1, 1)

	text, icon, num := st.SidebarItem, it.color, st.Count
	if selected {
		bg := m.ctx.Styles.Theme.Surface
		text, icon, num = st.SidebarSelected, it.color.Background(bg), st.CountSelected
		if !m.focused {
			text = text.Bold(false)
		}
	}
	return marker + text.Render(" ") + icon.Render(it.icon) +
		text.Render(" "+name+strings.Repeat(" ", gap)) + num.Render(count) + text.Render(" ")
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
