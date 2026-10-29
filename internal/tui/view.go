package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// Layout breakpoints (terminal columns).
const (
	sidebarWidth = 28
	narrowWidth  = 80 // below this only the focused pane is shown
	minWidth     = 40
	minHeight    = 10
	statusHeight = 1
)

// layout distributes the terminal among the panes.
func (m *Model) layout() {
	m.ctx.Width, m.ctx.Height = m.width, m.height
	h := m.height - statusHeight
	m.status.SetWidth(m.width)
	if m.narrow() {
		m.sidebar.SetSize(m.width, h)
		m.list.SetSize(m.width, h)
		return
	}
	m.sidebar.SetSize(sidebarWidth, h)
	m.list.SetSize(m.width-sidebarWidth, h)
}

func (m Model) narrow() bool { return m.width < narrowWidth }

// View implements tea.Model.
func (m Model) View() tea.View {
	th := m.ctx.Styles.Theme
	v := tea.NewView(fit(m.render(), m.width, m.height))
	v.AltScreen = true
	v.WindowTitle = "tasker · " + m.scope.Title
	// Paint the whole terminal with the theme so every cell, including
	// padding, matches; tea restores the original colors on exit.
	v.BackgroundColor = th.Base
	v.ForegroundColor = th.Text
	return v
}

func (m Model) render() string {
	if m.width == 0 {
		return "" // waiting for the first WindowSizeMsg
	}
	st := m.ctx.Styles
	if m.width < minWidth || m.height < minHeight {
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center,
			st.Warning.Render("Terminal too small")+"\n"+
				st.Muted.Render("need at least 40×10"))
	}

	var panes string
	switch {
	case m.narrow() && m.focus == paneSidebar:
		panes = m.sidebar.View()
	case m.narrow():
		panes = m.list.View()
	default:
		panes = lipgloss.JoinHorizontal(lipgloss.Top, m.sidebar.View(), m.list.View())
	}
	base := lipgloss.JoinVertical(lipgloss.Left, panes, m.status.View())
	if len(m.overlays) == 0 {
		return base
	}

	// Stack overlays above the base, each centered.
	layers := []*lipgloss.Layer{lipgloss.NewLayer(base)}
	for i, o := range m.overlays {
		box := o.View()
		w, h := lipgloss.Width(box), lipgloss.Height(box)
		x := max((m.width-w)/2, 0)
		y := max((m.height-h)/2, 0)
		layers = append(layers, lipgloss.NewLayer(box).X(x).Y(y).Z(i+1))
	}
	return lipgloss.NewCompositor(layers...).Render()
}

// fit clips or pads s to exactly w×h cells. Overlays larger than the
// terminal are clipped instead of scrolling the screen, and lines the
// compositor trimmed are padded back to full width.
func fit(s string, w, h int) string {
	if w <= 0 || h <= 0 {
		return s
	}
	lines := strings.Split(s, "\n")
	if len(lines) > h {
		lines = lines[:h]
	}
	for i, l := range lines {
		switch lw := ansi.StringWidth(l); {
		case lw > w:
			lines[i] = ansi.Truncate(l, w, "")
		case lw < w:
			lines[i] = l + strings.Repeat(" ", w-lw)
		}
	}
	for len(lines) < h {
		lines = append(lines, strings.Repeat(" ", w))
	}
	return strings.Join(lines, "\n")
}
