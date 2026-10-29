package pomodoro

import (
	"image/color"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/mahm0udnasr/tasker/internal/domain"
	"github.com/mahm0udnasr/tasker/internal/tui/common"
)

// phaseColor is red-ish for focus, green for breaks, muted when paused.
func (m *Model) phaseColor() color.Color {
	th := m.ctx.Styles.Theme
	switch {
	case m.Paused():
		return th.Muted
	case m.phase == Focus:
		return th.Priority[4]
	default:
		return th.Success
	}
}

func (m *Model) phaseIcon() string {
	switch {
	case m.Paused():
		return m.ctx.Icons.Pause
	case m.phase == Focus:
		return m.ctx.Icons.Tomato
	default:
		return m.ctx.Icons.Coffee
	}
}

// StatusSegment renders the timer for the status bar, e.g.
// "🍅 18:42 Finish ML ━━━━──". It is empty when idle.
func (m *Model) StatusSegment() string {
	if m.phase == Idle {
		return ""
	}
	st := m.ctx.Styles
	c := lipgloss.NewStyle().Foreground(m.phaseColor()).Bold(true)
	label := truncate(m.task.Title, 18)
	if m.phase != Focus {
		label = m.phase.Label()
	}
	if m.Paused() {
		label += " (paused)"
	}
	return c.Render(m.phaseIcon()+" "+domain.FormatClock(m.Remaining())) + " " +
		st.Subtext.Render(label) + " " + bar(m.Progress(), 10, m.phaseColor(), st.Theme.Surface)
}

// RowLabel returns the focused task's ID and a live label for its row.
func (m *Model) RowLabel() (taskID, label string) {
	if m.phase != Focus {
		return "", ""
	}
	icon := m.ctx.Icons.Play
	if m.Paused() {
		icon = m.ctx.Icons.Pause
	}
	return m.task.ID, lipgloss.NewStyle().Foreground(m.phaseColor()).Bold(true).
		Render(icon + " " + domain.FormatClock(m.Remaining()))
}

// FocusView renders the full-screen focus mode within w×h.
func (m *Model) FocusView(w, h int) string {
	st := m.ctx.Styles
	th := st.Theme
	col := m.phaseColor()
	center := lipgloss.NewStyle().Width(w).Align(lipgloss.Center)

	var lines []string
	if m.phase == Idle {
		lines = append(lines,
			st.ModalTitle.Render(m.ctx.Icons.Tomato+"  Focus mode"), "",
			bigClock(domain.FormatClock(m.opts.Settings.Focus), th.Muted), "",
			st.Subtext.Render(nonEmpty(m.task.Title, "No task selected")), "",
			st.KeyDesc.Render("P start · esc close"))
		return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, center.Render(strings.Join(lines, "\n")))
	}

	phase := strings.ToUpper(m.phase.Label())
	if m.Paused() {
		phase += " · PAUSED"
	}
	done, of := m.Cycle()
	dots := make([]string, of)
	for i := range dots {
		if i < done {
			dots[i] = lipgloss.NewStyle().Foreground(th.Priority[4]).Render("●")
		} else {
			dots[i] = st.Muted.Render("○")
		}
	}
	progress := m.task.PomodoroProgress()
	if progress != "" {
		progress = "  ·  " + m.ctx.Icons.Tomato + " " + progress + " on this task"
	}
	lines = append(lines,
		lipgloss.NewStyle().Foreground(col).Bold(true).Render(m.phaseIcon()+"  "+phase), "",
		bigClock(domain.FormatClock(m.Remaining()), col), "",
		bar(m.Progress(), min(w-8, 40), col, th.Surface), "",
		st.Bold.Render(truncate(m.task.Title, w-4)),
		st.Muted.Render(strings.Join(dots, " ")+progress), "",
		st.KeyDesc.Render("space pause · s skip · X stop · c complete task · esc close"),
	)
	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, center.Render(strings.Join(lines, "\n")))
}

// FocusOverlay shows FocusView as a near-full-screen modal in the TUI.
type FocusOverlay struct{ m *Model }

// NewFocusOverlay creates the focus-mode overlay.
func NewFocusOverlay(m *Model) *FocusOverlay { return &FocusOverlay{m: m} }

// HandleKey maps focus-mode keys to timer commands. It is shared with the
// standalone `tasker focus` program. ok is false for unhandled keys.
func HandleKey(m *Model, k tea.KeyPressMsg, selected *domain.Task) (cmd tea.Cmd, ok bool) {
	switch {
	case k.String() == "space" || m.ctx.Keys.Is(k, common.ActPomodoro):
		return m.Toggle(selected), true
	case k.String() == "s" || m.ctx.Keys.Is(k, common.ActPomoSkip):
		return m.Skip(), true
	case m.ctx.Keys.Is(k, common.ActPomoStop):
		return m.Stop(), true
	case k.String() == "c":
		return m.CompleteTask(), true
	}
	return nil, false
}

// Update implements common.Overlay.
func (o *FocusOverlay) Update(msg tea.Msg) (common.Overlay, tea.Cmd) {
	k, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return o, nil
	}
	if k.String() == "esc" || k.String() == "q" || o.m.ctx.Keys.Is(k, common.ActFocusMode) {
		return o, common.CloseOverlay
	}
	cmd, _ := HandleKey(o.m, k, nil)
	return o, cmd
}

// View implements common.Overlay.
func (o *FocusOverlay) View() string {
	w := max(min(o.m.ctx.Width-4, 76), 30)
	h := max(min(o.m.ctx.Height-4, 22), 14)
	return o.m.ctx.Styles.Modal.Padding(0, 1).BorderForeground(o.m.phaseColor()).
		Render(o.m.FocusView(w-4, h-2))
}

// --- drawing helpers ------------------------------------------------------------

func bar(p float64, w int, full, empty color.Color) string {
	w = max(w, 0)
	n := max(0, min(int(p*float64(w)+0.5), w))
	return lipgloss.NewStyle().Foreground(full).Render(strings.Repeat("━", n)) +
		lipgloss.NewStyle().Foreground(empty).Render(strings.Repeat("━", w-n))
}

// bigDigits is a 5-row block font for the focus-mode clock.
var bigDigits = map[rune][5]string{
	'0': {"█▀▀█", "█  █", "█  █", "█  █", "█▄▄█"},
	'1': {" ▄█ ", "  █ ", "  █ ", "  █ ", " ▄█▄"},
	'2': {"█▀▀█", "   █", "█▀▀▀", "█   ", "█▄▄▄"},
	'3': {"█▀▀█", "   █", " ▀▀█", "   █", "█▄▄█"},
	'4': {"█  █", "█  █", "▀▀▀█", "   █", "   █"},
	'5': {"█▀▀▀", "█   ", "▀▀▀█", "   █", "▄▄▄█"},
	'6': {"█▀▀▀", "█   ", "█▀▀█", "█  █", "█▄▄█"},
	'7': {"▀▀▀█", "   █", "  █ ", " █  ", " █  "},
	'8': {"█▀▀█", "█  █", "█▀▀█", "█  █", "█▄▄█"},
	'9': {"█▀▀█", "█  █", "▀▀▀█", "   █", "▄▄▄█"},
	':': {"  ", "▀ ", "  ", "▀ ", "  "},
}

func bigClock(s string, c color.Color) string {
	var rows [5][]string
	for _, r := range s {
		if g, ok := bigDigits[r]; ok {
			for i := range rows {
				rows[i] = append(rows[i], g[i])
			}
		}
	}
	style := lipgloss.NewStyle().Foreground(c)
	out := make([]string, len(rows))
	for i, glyphs := range rows {
		out[i] = style.Render(strings.Join(glyphs, " "))
	}
	return strings.Join(out, "\n")
}

func truncate(s string, n int) string {
	r := []rune(s)
	if n <= 1 || len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

func nonEmpty(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}
