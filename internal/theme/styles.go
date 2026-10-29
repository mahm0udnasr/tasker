package theme

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// Styles are the lipgloss styles derived from a Theme. They are rebuilt when
// the theme changes, so components should read them on every render rather
// than caching individual styles.
type Styles struct {
	Theme Theme

	Text, Subtext, Muted, Accent, Bold lipgloss.Style

	PanelTitle        lipgloss.Style
	PanelTitleFocused lipgloss.Style
	Border            lipgloss.Style
	BorderFocused     lipgloss.Style

	SidebarItem     lipgloss.Style
	SidebarSelected lipgloss.Style
	SidebarHeader   lipgloss.Style
	Count           lipgloss.Style
	CountSelected   lipgloss.Style

	Row            lipgloss.Style
	RowSelected    lipgloss.Style // focused pane
	RowSelectedDim lipgloss.Style // selection in an unfocused pane
	DoneTitle      lipgloss.Style
	Tag            lipgloss.Style
	Project        lipgloss.Style
	DueOverdue     lipgloss.Style
	DueToday       lipgloss.Style
	DueLater       lipgloss.Style

	StatusBar   lipgloss.Style
	StatusPill  lipgloss.Style
	StatusError lipgloss.Style
	StatusInfo  lipgloss.Style
	KeyHint     lipgloss.Style
	KeyDesc     lipgloss.Style

	Modal      lipgloss.Style
	ModalTitle lipgloss.Style
	Danger     lipgloss.Style
	Success    lipgloss.Style
	Warning    lipgloss.Style
	Empty      lipgloss.Style
}

// NewStyles derives styles from t.
func NewStyles(t Theme) *Styles {
	n := lipgloss.NewStyle
	s := &Styles{Theme: t}
	s.Text = n().Foreground(t.Text)
	s.Subtext = n().Foreground(t.Subtext)
	s.Muted = n().Foreground(t.Muted)
	s.Accent = n().Foreground(t.Accent)
	s.Bold = n().Foreground(t.Text).Bold(true)

	s.PanelTitle = n().Foreground(t.Subtext).Bold(true)
	s.PanelTitleFocused = n().Foreground(t.Accent).Bold(true)
	s.Border = n().Foreground(t.Border)
	s.BorderFocused = n().Foreground(t.Accent)

	s.SidebarItem = n().Foreground(t.Subtext)
	s.SidebarSelected = n().Foreground(t.Text).Background(t.Surface).Bold(true)
	s.SidebarHeader = n().Foreground(t.Muted).Bold(true)
	s.Count = n().Foreground(t.Muted)
	s.CountSelected = n().Foreground(t.Accent).Background(t.Surface).Bold(true)

	s.Row = n().Foreground(t.Text)
	s.RowSelected = n().Foreground(t.Text).Background(t.Surface)
	s.RowSelectedDim = n().Foreground(t.Subtext).Background(t.Surface)
	s.DoneTitle = n().Foreground(t.Muted).Strikethrough(true)
	s.Tag = n().Foreground(t.Tag)
	s.Project = n().Foreground(t.Secondary)
	s.DueOverdue = n().Foreground(t.Danger).Bold(true)
	s.DueToday = n().Foreground(t.Warning).Bold(true)
	s.DueLater = n().Foreground(t.Subtext)

	s.StatusBar = n().Foreground(t.Subtext)
	s.StatusPill = n().Foreground(t.Base).Background(t.Accent).Bold(true).Padding(0, 1)
	s.StatusError = n().Foreground(t.Danger).Bold(true)
	s.StatusInfo = n().Foreground(t.Success)
	s.KeyHint = n().Foreground(t.Accent)
	s.KeyDesc = n().Foreground(t.Muted)

	// Modals rely on the terminal background (set to Base by the app view):
	// a lipgloss Background here would be punched through by the inner
	// text's ANSI resets.
	s.Modal = n().Border(lipgloss.RoundedBorder()).BorderForeground(t.Accent).Padding(1, 2)
	s.ModalTitle = n().Foreground(t.Accent).Bold(true)
	s.Danger = n().Foreground(t.Danger).Bold(true)
	s.Success = n().Foreground(t.Success)
	s.Warning = n().Foreground(t.Warning)
	s.Empty = n().Foreground(t.Muted).Italic(true)
	return s
}

// Panel draws a rounded box of exactly w×h cells with the title set into the
// top border ("╭─ Inbox ─────╮"). lipgloss has no border titles, so the frame
// is assembled by hand; content is clipped/padded to fit.
func (s *Styles) Panel(title, content string, w, h int, focused bool) string {
	if w < 4 || h < 2 {
		return ""
	}
	border, titleStyle := s.Border, s.PanelTitle
	if focused {
		border, titleStyle = s.BorderFocused, s.PanelTitleFocused
	}
	b := lipgloss.RoundedBorder()
	inner := w - 2

	var top string
	if title != "" && inner > 6 {
		t := titleStyle.Render(" " + truncate(title, inner-4) + " ")
		fill := max(inner-1-lipgloss.Width(t), 0)
		top = border.Render(b.TopLeft+b.Top) + t + border.Render(strings.Repeat(b.Top, fill)+b.TopRight)
	} else {
		top = border.Render(b.TopLeft + strings.Repeat(b.Top, inner) + b.TopRight)
	}

	body := lipgloss.NewStyle().Width(inner).Height(h - 2).MaxHeight(h - 2).MaxWidth(inner).Render(content)
	lines := strings.Split(body, "\n")
	out := make([]string, 0, h)
	out = append(out, top)
	side := border.Render(b.Left)
	sideR := border.Render(b.Right)
	for _, l := range lines {
		out = append(out, side+l+strings.Repeat(" ", max(inner-lipgloss.Width(l), 0))+sideR)
	}
	out = append(out, border.Render(b.BottomLeft+strings.Repeat(b.Bottom, inner)+b.BottomRight))
	return strings.Join(out, "\n")
}

func truncate(s string, w int) string {
	if lipgloss.Width(s) <= w {
		return s
	}
	r := []rune(s)
	for len(r) > 0 && lipgloss.Width(string(r))+1 > w {
		r = r[:len(r)-1]
	}
	return string(r) + "…"
}
