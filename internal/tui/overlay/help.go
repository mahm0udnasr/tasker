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
	{"Tasks", []string{common.ActAdd, common.ActAddSubtask, common.ActEdit, common.ActNotes, common.ActToggleDone,
		common.ActToggleStart, common.ActDelete, common.ActPriority, common.ActDue, common.ActMove,
		common.ActMoveUp, common.ActMoveDown}},
	{"Pomodoro", []string{common.ActPomodoro, common.ActPomoStop, common.ActPomoSkip, common.ActFocusMode}},
	{"General", []string{common.ActStats, common.ActTheme, common.ActSearch, common.ActPalette, common.ActFilter, common.ActClearFilter,
		common.ActSort, common.ActUndo, common.ActDetail, common.ActDelProject, common.ActRefresh,
		common.ActHelp, common.ActQuit}},
}

// Keys inside the task form and the notes editor are fixed, not rebindable.
var (
	helpForm = [][2]string{
		{"tab", "next field / complete name"},
		{"shift+tab", "previous field"},
		{"↑/↓", "other project / tag matches"},
		{"ctrl+s", "save"},
		{"esc", "cancel"},
	}
	helpNotes = [][2]string{
		{"enter", "new line"},
		{"ctrl+e", "open in $EDITOR"},
		{"ctrl+s", "save"},
		{"esc", "cancel"},
	}
)

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
	column := func(title string, rows [][2]string) string {
		keyW := 0
		for _, r := range rows {
			keyW = max(keyW, lipgloss.Width(r[0]))
		}
		lines := []string{st.ModalTitle.Render(title)}
		for _, r := range rows {
			lines = append(lines, st.KeyHint.Render(r[0]+strings.Repeat(" ", keyW-lipgloss.Width(r[0])))+"  "+
				st.Subtext.Render(r[1]))
		}
		return strings.Join(lines, "\n")
	}
	var cols []string
	for _, sec := range helpSections {
		rows := make([][2]string, len(sec.actions))
		for i, a := range sec.actions {
			b := h.ctx.Keys.Get(a).Help()
			rows[i] = [2]string{b.Key, b.Desc}
		}
		cols = append(cols, column(sec.title, rows))
	}
	form, notes := column("Task form", helpForm), column("Notes editor", helpNotes)
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
	body := lipgloss.JoinHorizontal(lipgloss.Top, v(nav, form), gap, v(tasks, notes), gap, v(pomo, general))
	if lipgloss.Width(body)+8 > h.ctx.Width {
		body = lipgloss.JoinHorizontal(lipgloss.Top, v(nav, pomo, form, notes), gap, v(tasks, general))
	}
	if lipgloss.Width(body)+8 > h.ctx.Width {
		body = v(nav, tasks, pomo, general, form, notes)
	}
	footer := st.KeyDesc.Render("esc to close · rebind keys in config.toml")
	return st.Modal.Padding(1, 2).Render(lipgloss.JoinVertical(lipgloss.Left,
		st.Bold.Render("Keyboard shortcuts"), "", body, "", footer))
}
