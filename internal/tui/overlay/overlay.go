// Package overlay contains the small modal components: confirm dialog,
// single-line prompt, fuzzy picker and help screen.
package overlay

import (
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/mahm0udnasr/tasker/internal/tui/common"
)

// newInput returns a text input styled with the current theme.
func newInput(ctx *common.Context, placeholder string, width int) textinput.Model {
	th := ctx.Styles.Theme
	ti := textinput.New()
	ti.Prompt = "❯ "
	ti.Placeholder = placeholder
	ti.SetWidth(width)
	s := textinput.DefaultStyles(th.Dark)
	s.Focused.Prompt = ctx.Styles.Accent
	s.Focused.Text = ctx.Styles.Text
	s.Focused.Placeholder = ctx.Styles.Muted
	s.Focused.Suggestion = ctx.Styles.Muted
	s.Blurred = s.Focused
	s.Cursor.Color = th.Accent
	ti.SetStyles(s)
	return ti
}

// closeWith closes the overlay and then delivers msg.
func closeWith(msg tea.Msg) tea.Cmd {
	if msg == nil {
		return common.CloseOverlay
	}
	return tea.Sequence(common.CloseOverlay, func() tea.Msg { return msg })
}

func isEsc(msg tea.KeyPressMsg) bool { return msg.String() == "esc" }
