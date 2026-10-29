// Package statusbar renders the bottom bar: current view, counts, transient
// messages, today's progress and key hints.
package statusbar

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/progress"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/mahm0udnasr/tasker/internal/domain"
	"github.com/mahm0udnasr/tasker/internal/tui/common"
)

const messageTTL = 4 * time.Second

type clearMsg struct{ id int }

// Model is the status bar.
type Model struct {
	ctx      *common.Context
	width    int
	view     string
	counts   domain.Counts
	msg      string
	isErr    bool
	msgID    int
	hints    []key.Binding
	progress progress.Model
	timer    string // pomodoro segment; replaces the progress bar when set
}

// New creates a status bar.
func New(ctx *common.Context) Model {
	m := Model{ctx: ctx}
	m.ApplyTheme()
	return m
}

// ApplyTheme rebuilds the progress bar colors from the current theme.
func (m *Model) ApplyTheme() {
	th := m.ctx.Styles.Theme
	m.progress = progress.New(
		progress.WithWidth(12),
		progress.WithoutPercentage(),
		progress.WithColors(th.Accent, th.Success),
		progress.WithFillCharacters('━', '━'),
	)
	m.progress.EmptyColor = th.Surface
}

// SetWidth sets the bar width.
func (m *Model) SetWidth(w int) { m.width = w }

// SetView sets the current view label.
func (m *Model) SetView(v string) { m.view = v }

// SetCounts updates counts used for the summary and progress.
func (m *Model) SetCounts(c domain.Counts) { m.counts = c }

// SetTimer shows a running pomodoro instead of today's progress.
func (m *Model) SetTimer(s string) { m.timer = s }

// SetHints sets the key hints shown on the right.
func (m *Model) SetHints(h []key.Binding) { m.hints = h }

// Update handles status messages and their expiry.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case common.StatusMsg:
		m.msgID++
		m.msg, m.isErr = msg.Text, msg.Error
		id := m.msgID
		ttl := messageTTL
		if msg.Error {
			ttl *= 2 // errors stay a little longer
		}
		return m, common.Tick(ttl, func(time.Time) tea.Msg { return clearMsg{id: id} })
	case clearMsg:
		if msg.id == m.msgID {
			m.msg = ""
		}
	}
	return m, nil
}

// View renders a single line of exactly m.width cells.
func (m Model) View() string {
	st := m.ctx.Styles
	ic := m.ctx.Icons
	left := st.StatusPill.Render(strings.ToUpper(m.view))

	var mid string
	switch {
	case m.msg != "" && m.isErr:
		mid = st.StatusError.Render(ic.Cross + " " + m.msg)
	case m.msg != "":
		mid = st.StatusInfo.Render(ic.Check + " " + m.msg)
	default:
		parts := []string{st.Subtext.Render(fmt.Sprintf("%d today", m.counts.Today))}
		if m.counts.Overdue > 0 {
			parts = append(parts, st.Danger.Render(fmt.Sprintf("%d overdue", m.counts.Overdue)))
		}
		parts = append(parts, st.Muted.Render(fmt.Sprintf("%d done today", m.counts.DoneToday)))
		mid = strings.Join(parts, st.Muted.Render(" · "))
	}

	pct := m.counts.TodayProgress()
	prog := st.Muted.Render("today ") + m.progress.ViewAs(pct) +
		st.Subtext.Render(fmt.Sprintf(" %3.0f%%", pct*100))
	if m.timer != "" {
		prog = m.timer
	}

	var hints []string
	for _, h := range m.hints {
		hints = append(hints, st.KeyHint.Render(h.Help().Key)+" "+st.KeyDesc.Render(h.Help().Desc))
	}
	right := strings.Join(hints, st.Muted.Render("  "))

	// Drop pieces from the right as the terminal narrows.
	avail := m.width - lipgloss.Width(left) - 2
	for _, candidate := range []string{prog + "   " + right, prog, ""} {
		if lipgloss.Width(mid)+lipgloss.Width(candidate)+2 <= avail || candidate == "" {
			right = candidate
			break
		}
	}
	if w := lipgloss.Width(mid); w > avail-lipgloss.Width(right)-1 {
		mid = truncate(mid, max(avail-lipgloss.Width(right)-1, 0))
	}
	gap := max(avail-lipgloss.Width(mid)-lipgloss.Width(right), 1)
	return left + " " + mid + strings.Repeat(" ", gap) + right + " "
}

// truncate shortens a styled string to w cells.
func truncate(s string, w int) string {
	return lipgloss.NewStyle().MaxWidth(w).Render(s)
}
