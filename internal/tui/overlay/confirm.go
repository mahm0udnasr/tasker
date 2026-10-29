package overlay

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/mahm0udnasr/tasker/internal/tui/common"
)

// Confirm asks a yes/no question; "yes" delivers Action.
type Confirm struct {
	ctx    *common.Context
	title  string
	body   string
	action tea.Msg
	yes    bool // which button is highlighted
}

// NewConfirm builds a destructive-action dialog. The highlighted default is
// "No" so a stray Enter never deletes anything.
func NewConfirm(ctx *common.Context, title, body string, action tea.Msg) *Confirm {
	return &Confirm{ctx: ctx, title: title, body: body, action: action}
}

// Update implements common.Overlay.
func (c *Confirm) Update(msg tea.Msg) (common.Overlay, tea.Cmd) {
	km, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return c, nil
	}
	switch km.String() {
	case "y", "Y":
		return c, closeWith(c.action)
	case "n", "N", "esc", "q":
		return c, common.CloseOverlay
	case "left", "right", "h", "l", "tab", "shift+tab":
		c.yes = !c.yes
	case "enter":
		if c.yes {
			return c, closeWith(c.action)
		}
		return c, common.CloseOverlay
	}
	return c, nil
}

// View implements common.Overlay.
func (c *Confirm) View() string {
	st := c.ctx.Styles
	th := st.Theme
	btn := lipgloss.NewStyle().Padding(0, 2)
	no := btn.Foreground(th.Text).Background(th.Surface)
	yes := btn.Foreground(th.Subtext).Background(th.Surface)
	if c.yes {
		yes = btn.Foreground(th.Base).Background(th.Danger).Bold(true)
	} else {
		no = btn.Foreground(th.Base).Background(th.Accent).Bold(true)
	}
	buttons := lipgloss.JoinHorizontal(lipgloss.Top, no.Render("No"), "  ", yes.Render("Yes, delete"))
	content := lipgloss.JoinVertical(lipgloss.Left,
		st.Danger.Render(c.ctx.Icons.Warn+"  "+c.title),
		"",
		lipgloss.NewStyle().Width(46).Foreground(th.Text).Render(c.body),
		"",
		buttons,
		"",
		st.KeyDesc.Render("y/n · ←/→ to choose · enter to confirm"),
	)
	return st.Modal.BorderForeground(th.Danger).Render(content)
}
