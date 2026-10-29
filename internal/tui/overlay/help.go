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
	{"General", []string{common.ActSearch, common.ActPalette, common.ActFilter, common.ActClearFilter,
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
	// Widest layout that fits: all sections side by side, then two columns
	// with the last sections stacked, then a single column.
	body := lipgloss.JoinHorizontal(lipgloss.Top, cols[0], gap, cols[1], gap, cols[2])
	if lipgloss.Width(body)+8 > h.ctx.Width {
		body = lipgloss.JoinHorizontal(lipgloss.Top, cols[0], gap,
			lipgloss.JoinVertical(lipgloss.Left, cols[1], "", cols[2]))
	}
	if lipgloss.Width(body)+8 > h.ctx.Width {
		body = lipgloss.JoinVertical(lipgloss.Left, cols[0], "", cols[1], "", cols[2])
	}
	footer := st.KeyDesc.Render("esc to close · rebind keys in config.toml")
	return st.Modal.Padding(1, 2).Render(lipgloss.JoinVertical(lipgloss.Left,
		st.Bold.Render("Keyboard shortcuts"), "", body, "", footer))
}
