package overlay

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/mahm0udnasr/tasker/internal/domain"
	"github.com/mahm0udnasr/tasker/internal/tui/common"
)

// RefineMsg applies a new filter.
type RefineMsg struct{ Refinement domain.Refinement }

type filterRow struct {
	label   string
	options []string
	sel     int
}

// Filter edits a domain.Refinement: one row per dimension, ←/→ to change.
type Filter struct {
	ctx    *common.Context
	rows   []filterRow
	cursor int
}

var (
	filterPriorities = []domain.Priority{domain.PriorityNone, domain.PriorityLow, domain.PriorityMedium,
		domain.PriorityHigh, domain.PriorityUrgent}
	filterDue      = []domain.DueFilter{domain.DueAny, domain.DueSet, domain.DueNone, domain.DueOverdue}
	filterStatuses = []domain.Status{"", domain.StatusTodo, domain.StatusInProgress}
)

// NewFilter opens the filter editor seeded with the current refinement.
func NewFilter(ctx *common.Context, r domain.Refinement) *Filter {
	f := &Filter{ctx: ctx, rows: []filterRow{
		{label: "Priority", options: []string{"any", "low+", "medium+", "high+", "urgent"}},
		{label: "Due date", options: []string{"any", "has a date", "no date", "overdue"}},
		{label: "Status", options: []string{"any", "to do", "in progress"}},
	}}
	f.rows[0].sel = indexOf(filterPriorities, r.MinPriority)
	f.rows[1].sel = indexOf(filterDue, r.Due)
	f.rows[2].sel = indexOf(filterStatuses, r.Status)
	return f
}

func indexOf[T comparable](xs []T, x T) int {
	for i, v := range xs {
		if v == x {
			return i
		}
	}
	return 0
}

func (f *Filter) value() domain.Refinement {
	return domain.Refinement{
		MinPriority: filterPriorities[f.rows[0].sel],
		Due:         filterDue[f.rows[1].sel],
		Status:      filterStatuses[f.rows[2].sel],
	}
}

// Update implements common.Overlay.
func (f *Filter) Update(msg tea.Msg) (common.Overlay, tea.Cmd) {
	km, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return f, nil
	}
	row := &f.rows[f.cursor]
	switch km.String() {
	case "esc", "q":
		return f, common.CloseOverlay
	case "enter":
		return f, closeWith(RefineMsg{Refinement: f.value()})
	case "c", "backspace":
		return f, closeWith(RefineMsg{})
	case "up", "k", "shift+tab":
		f.cursor = (f.cursor + len(f.rows) - 1) % len(f.rows)
	case "down", "j", "tab":
		f.cursor = (f.cursor + 1) % len(f.rows)
	case "left", "h":
		row.sel = (row.sel + len(row.options) - 1) % len(row.options)
	case "right", "l", "space":
		row.sel = (row.sel + 1) % len(row.options)
	}
	return f, nil
}

// View implements common.Overlay.
func (f *Filter) View() string {
	st := f.ctx.Styles
	th := st.Theme
	lines := []string{st.ModalTitle.Render("Filter this list"), ""}
	for i, r := range f.rows {
		label := st.Subtext.Render(padRight(r.label, 10))
		if i == f.cursor {
			label = st.Accent.Bold(true).Render(padRight(r.label, 10))
		}
		var opts []string
		for j, o := range r.options {
			style := lipgloss.NewStyle().Foreground(th.Muted).Padding(0, 1)
			if j == r.sel {
				style = style.Foreground(th.Base).Background(th.Accent).Bold(true)
				if i != f.cursor {
					style = style.Background(th.Subtext)
				}
			}
			opts = append(opts, style.Render(o))
		}
		lines = append(lines, label+strings.Join(opts, ""), "")
	}
	summary := f.value().Describe()
	if summary == "" {
		summary = "showing everything"
	}
	lines = append(lines, st.Muted.Render("→ "+summary), "",
		st.KeyDesc.Render("↑/↓ row · ←/→ change · enter apply · c clear · esc cancel"))
	return st.Modal.Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
}

func padRight(s string, w int) string {
	if n := w - lipgloss.Width(s); n > 0 {
		return s + strings.Repeat(" ", n)
	}
	return s
}
