package overlay

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/mahm0udnasr/tasker/internal/tui/common"
)

// WelcomeDoneMsg is sent when the welcome screen closes; Samples asks for
// the example tasks.
type WelcomeDoneMsg struct{ Samples bool }

// Welcome is the first-run screen.
type Welcome struct{ ctx *common.Context }

// NewWelcome creates the welcome screen.
func NewWelcome(ctx *common.Context) *Welcome { return &Welcome{ctx: ctx} }

// Update implements common.Overlay: s loads samples, any other key closes.
func (w *Welcome) Update(msg tea.Msg) (common.Overlay, tea.Cmd) {
	k, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return w, nil
	}
	return w, closeWith(WelcomeDoneMsg{Samples: k.String() == "s"})
}

const logo = `▀█▀ ▄▀█ █▀ █▄▀ █▀▀ █▀█
 █  █▀█ ▄█ █ █ ██▄ █▀▄`

// View implements common.Overlay.
func (w *Welcome) View() string {
	st := w.ctx.Styles
	k := w.ctx.Keys
	key := func(action, desc string) string {
		return st.KeyHint.Render(lipgloss.NewStyle().Width(8).Render(k.Get(action).Help().Key)) + st.Subtext.Render(desc)
	}
	body := lipgloss.JoinVertical(lipgloss.Left,
		st.Accent.Bold(true).Render(logo),
		"",
		st.Bold.Render("Welcome! A few keys to get you going:"),
		"",
		key(common.ActAdd, "add a task — dates like “tomorrow 5pm” just work"),
		key(common.ActToggleDone, "complete it"),
		key(common.ActPomodoro, "start a pomodoro on it"),
		key(common.ActSearch, "search everything"),
		key(common.ActPalette, "every command, one keystroke away"),
		key(common.ActTheme, "pick a theme"),
		key(common.ActHelp, "all shortcuts"),
		"",
		st.Muted.Render("Your settings live in ~/.config/tasker/config.toml"),
		"",
		st.Success.Render("s")+st.Subtext.Render(" add a few example tasks  ·  ")+
			st.KeyHint.Render("any key")+st.Subtext.Render(" start fresh"),
	)
	return st.Modal.Padding(1, 3).Render(body)
}
