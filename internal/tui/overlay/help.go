package overlay

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/mahm0udnasr/tasker/internal/tui/common"
)

// Help shows every key binding, grouped.
type Help struct{ ctx *common.Context }

// NewHelp creates the help screen.
func NewHelp(ctx *common.Context) *Help { return &Help{ctx: ctx} }

var helpSections = []struct {
	title   string
	actions []string
}{
	{"Navigate", []string{common.ActUp, common.ActDown, common.ActTop, common.ActBottom,
		common.ActPageUp, common.ActPageDown, common.ActLeft, common.ActRight, common.ActNextPane, common.ActExpand}},
	{"Tasks", []string{common.ActAdd, common.ActAddSubtask, common.ActEdit, common.ActToggleDone,
		common.ActToggleStart, common.ActDelete, common.ActPriority, common.ActDue, common.ActMove,
		common.ActMoveUp, common.ActMoveDown}},
	{"Pomodoro", []string{common.ActPomodoro, common.ActPomoStop, common.ActPomoSkip, common.ActFocusMode}},
	{"General", []string{common.ActStats, common.ActTheme, common.ActSearch, common.ActPalette, common.ActFilter, common.ActClearFilter,
		common.ActSort, common.ActUndo, common.ActDetail, common.ActDelProject, common.ActRefresh,
		common.ActHelp, common.ActQuit}},
}

// Update implements common.Overlay: any of esc, ?, q closes.
func (h *Help) Update(msg tea.Msg) (common.Overlay, tea.Cmd) {
	if km, ok := msg.(tea.KeyPressMsg); ok {
		if isEsc(km) || h.ctx.Keys.Is(km, common.ActHelp) || km.String() == "q" {
			return h, common.CloseOverlay
		}
	}
	return h, nil
}

// View implements common.Overlay.
func (h *Help) View() string {
	st := h.ctx.Styles
	var cols []string
	for _, sec := range helpSections {
		keyW := 0
		for _, a := range sec.actions {
			keyW = max(keyW, lipgloss.Width(h.ctx.Keys.Get(a).Help().Key))
		}
		lines := []string{st.ModalTitle.Render(sec.title)}
		for _, a := range sec.actions {
			b := h.ctx.Keys.Get(a)
			k := b.Help().Key
			lines = append(lines, st.KeyHint.Render(k+strings.Repeat(" ", keyW-lipgloss.Width(k)))+"  "+
				st.Subtext.Render(b.Help().Desc))
		}
		cols = append(cols, strings.Join(lines, "\n"))
	}
	gap := "    "
	v := func(parts ...string) string {
		var withGaps []string
		for i, p := range parts {
			if i > 0 {
				withGaps = append(withGaps, "")
			}
			withGaps = append(withGaps, p)
		}
		return lipgloss.JoinVertical(lipgloss.Left, withGaps...)
	}
	// Widest layout that fits: three columns, then two, then one.
	nav, tasks, pomo, general := cols[0], cols[1], cols[2], cols[3]
	body := lipgloss.JoinHorizontal(lipgloss.Top, nav, gap, tasks, gap, v(pomo, general))
	if lipgloss.Width(body)+8 > h.ctx.Width {
		body = lipgloss.JoinHorizontal(lipgloss.Top, v(nav, pomo), gap, v(tasks, general))
	}
	if lipgloss.Width(body)+8 > h.ctx.Width {
		body = v(nav, tasks, pomo, general)
	}
	footer := st.KeyDesc.Render("esc to close · rebind keys in config.toml")
	return st.Modal.Padding(1, 2).Render(lipgloss.JoinVertical(lipgloss.Left,
		st.Bold.Render("Keyboard shortcuts"), "", body, "", footer))
}
