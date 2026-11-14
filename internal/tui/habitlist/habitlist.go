// Package habitlist renders the habits view: one row per habit with
// today's status, the current streak and a strip of the last days. Streaks
// are computed from the logs on every render, so they roll over at midnight.
package habitlist

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/mahm0udnasr/tasker/internal/domain"
	"github.com/mahm0udnasr/tasker/internal/tui/common"
)

// StripDays is how many days the history strip shows, ending today.
const StripDays = 7

// Row is one habit and the days it was done (keyed by domain.DayKey).
type Row struct {
	Habit domain.Habit
	Done  map[string]bool
}

// DoneToday reports whether the habit is done on now's day.
func (r Row) DoneToday(ctx *common.Context) bool { return r.Done[domain.DayKey(ctx.Now())] }

// Model is the habit list.
type Model struct {
	ctx     *common.Context
	rows    []Row
	cursor  int
	offset  int
	width   int
	height  int
	focused bool
	loaded  bool
}

// New creates an empty list.
func New(ctx *common.Context) Model { return Model{ctx: ctx} }

// SetRows replaces the rows, keeping the cursor on the same habit if it is
// still present.
func (m *Model) SetRows(rows []Row) {
	prevID := ""
	if r, ok := m.Selected(); ok {
		prevID = r.Habit.ID
	}
	m.rows = rows
	m.loaded = true
	if prevID != "" {
		m.SelectID(prevID)
	}
	m.clamp()
}

// SelectID moves the cursor to the habit with id, if present.
func (m *Model) SelectID(id string) {
	for i, r := range m.rows {
		if r.Habit.ID == id {
			m.cursor = i
			m.clamp()
			return
		}
	}
}

// Selected returns the row under the cursor.
func (m Model) Selected() (Row, bool) {
	if m.cursor < 0 || m.cursor >= len(m.rows) {
		return Row{}, false
	}
	return m.rows[m.cursor], true
}

// OpenToday counts habits not yet done today.
func (m Model) OpenToday() int {
	n := 0
	for _, r := range m.rows {
		if !r.DoneToday(m.ctx) {
			n++
		}
	}
	return n
}

// Len is the number of habits.
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

// Update handles navigation.
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
	default:
		return m, nil
	}
	m.clamp()
	return m, nil
}

// View renders the panel.
func (m Model) View() string {
	st := m.ctx.Styles
	inner := m.width - 2
	title := "Habits"
	if m.loaded {
		title = fmt.Sprintf("Habits · %d", len(m.rows))
		if len(m.rows) > 0 {
			title += fmt.Sprintf(" · %d/%d today", len(m.rows)-m.OpenToday(), len(m.rows))
		}
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
		st.Accent.Render(m.ctx.Icons.Habits),
		"",
		st.Empty.Render("No habits yet. Start with something small."),
		"",
		st.KeyDesc.Render("press ")+st.KeyHint.Render(m.ctx.Keys.Get(common.ActAdd).Help().Key)+
			st.KeyDesc.Render(" to add a habit"),
	)
	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, msg)
}

// renderRow lays out one row in exactly w cells:
//
//	▌ ○ Name……………………………  🔥 12   ● ● ○ ● ● ● ○
//
// The strip runs oldest → today. As in the task list, every segment is
// painted with the row background when selected.
func (m Model) renderRow(r Row, selected bool, w int) string {
	st := m.ctx.Styles
	ic := m.ctx.Icons
	th := st.Theme
	now := m.ctx.Now()

	paint := func(s lipgloss.Style) lipgloss.Style {
		if selected {
			return s.Background(th.Surface)
		}
		return s
	}
	plain := paint(lipgloss.NewStyle())

	cursor := " "
	if selected && m.focused {
		cursor = st.Accent.Render(ic.Selected)
	}
	status := paint(st.Muted).Render(ic.Todo)
	if r.DoneToday(m.ctx) {
		status = paint(st.Success).Render(ic.Done)
	}
	left := cursor + plain.Render(" ") + status + plain.Render(" ")

	streak := domain.HabitStreak(r.Done, now)
	ss := paint(st.Muted)
	if streak > 0 {
		ss = paint(st.Warning)
	}
	streakStr := ss.Render(fmt.Sprintf("%s %-3d", ic.Flame, streak))

	dots := make([]string, 0, StripDays)
	for _, done := range domain.HabitDays(r.Done, now, StripDays) {
		if done {
			dots = append(dots, paint(st.Success).Render(ic.DayDone))
		} else {
			dots = append(dots, paint(st.Muted).Render(ic.DayMissed))
		}
	}
	strip := strings.Join(dots, plain.Render(" "))

	// The strip goes first when space runs out, then the streak.
	const minName = 12
	right := streakStr + plain.Render("  ") + strip
	if w-lipgloss.Width(left)-lipgloss.Width(right)-2 < minName {
		right = streakStr
	}
	if w-lipgloss.Width(left)-lipgloss.Width(right)-2 < minName {
		right = ""
	}
	nameW := w - lipgloss.Width(left) - lipgloss.Width(right) - 2
	ns := paint(st.Row)
	if selected {
		ns = ns.Bold(true)
	}
	name := truncate(r.Habit.Name, nameW)
	gap := max(w-lipgloss.Width(left)-lipgloss.Width(name)-lipgloss.Width(right)-1, 1)
	return left + ns.Render(name) + plain.Render(strings.Repeat(" ", gap)) + right + plain.Render(" ")
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
