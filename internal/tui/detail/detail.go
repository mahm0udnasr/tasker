// Package detail renders the selected task in full: metadata, subtasks and
// the description as markdown, in a scrollable viewport.
package detail

import (
	"fmt"
	"image/color"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/glamour/v2"
	gansi "charm.land/glamour/v2/ansi"
	"charm.land/glamour/v2/styles"
	"charm.land/lipgloss/v2"

	"github.com/mahm0udnasr/tasker/internal/domain"
	"github.com/mahm0udnasr/tasker/internal/tui/common"
)

// Model is the detail pane.
type Model struct {
	ctx      *common.Context
	vp       viewport.Model
	task     *domain.Task
	subtasks []domain.Task
	project  domain.Project
	width    int
	height   int
	focused  bool

	// The glamour renderer is costly to build; reuse it while the width and
	// theme stay the same.
	renderer      *glamour.TermRenderer
	rendererWidth int
	rendererTheme string
}

// New creates an empty detail pane.
func New(ctx *common.Context) Model {
	vp := viewport.New()
	vp.KeyMap = viewport.KeyMap{
		Up:           key.NewBinding(key.WithKeys("k", "up")),
		Down:         key.NewBinding(key.WithKeys("j", "down")),
		HalfPageUp:   key.NewBinding(key.WithKeys("ctrl+u")),
		HalfPageDown: key.NewBinding(key.WithKeys("ctrl+d")),
		PageUp:       key.NewBinding(key.WithKeys("pgup")),
		PageDown:     key.NewBinding(key.WithKeys("pgdown")),
	}
	return Model{ctx: ctx, vp: vp}
}

// SetTask shows t (nil clears the pane). The scroll position is kept when
// the same task is refreshed, and reset when a different one is selected.
func (m *Model) SetTask(t *domain.Task, subtasks []domain.Task, project domain.Project) {
	if t == nil || m.task == nil || t.ID != m.task.ID {
		m.vp.GotoTop()
	}
	m.task, m.subtasks, m.project = t, subtasks, project
	m.refresh()
}

// TaskID returns the ID of the shown task, or "".
func (m Model) TaskID() string {
	if m.task == nil {
		return ""
	}
	return m.task.ID
}

// SetSize sets the outer size including the border.
func (m *Model) SetSize(w, h int) {
	m.width, m.height = w, h
	m.vp.SetWidth(max(w-4, 1))
	m.vp.SetHeight(max(h-2, 1))
	m.refresh()
}

// SetFocused toggles focus styling.
func (m *Model) SetFocused(f bool) { m.focused = f }

// Update scrolls the viewport.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmd tea.Cmd
	m.vp, cmd = m.vp.Update(msg)
	return m, cmd
}

// View renders the pane with a border.
func (m Model) View() string {
	title := "Details"
	if m.task != nil && m.vp.TotalLineCount() > m.vp.Height() {
		title = fmt.Sprintf("Details · %d%%", int(m.vp.ScrollPercent()*100))
	}
	body := lipgloss.NewStyle().PaddingLeft(1).Render(m.vp.View())
	return m.ctx.Styles.Panel(title, body, m.width, m.height, m.focused)
}

// Content renders the body without a border, for the modal variant.
func (m Model) Content() string { return m.vp.View() }

func (m *Model) refresh() {
	if m.width == 0 {
		return
	}
	m.vp.SetContent(m.render(m.vp.Width()))
}

func (m *Model) render(w int) string {
	st := m.ctx.Styles
	ic := m.ctx.Icons
	if m.task == nil {
		return lipgloss.Place(w, max(m.vp.Height(), 3), lipgloss.Center, lipgloss.Center,
			st.Empty.Render("Select a task to see its details"))
	}
	t := m.task
	now := m.ctx.Now()
	wrap := lipgloss.NewStyle().Width(w)

	var b strings.Builder
	title := st.Bold
	if t.IsDone() {
		title = st.DoneTitle
	}
	b.WriteString(wrap.Render(title.Render(t.Title)))
	b.WriteString("\n\n")

	field := func(icon, label, value string) {
		// Icons differ in width (the tomato emoji takes two cells): pad to 2.
		icon += strings.Repeat(" ", max(2-lipgloss.Width(icon), 0))
		b.WriteString(st.Muted.Render(fmt.Sprintf("%s %-9s", icon, label)))
		b.WriteString(value)
		b.WriteString("\n")
	}
	status := map[domain.Status]string{
		domain.StatusTodo:       st.Subtext.Render(ic.Todo + " to do"),
		domain.StatusInProgress: st.Warning.Render(ic.InProgress + " in progress"),
		domain.StatusDone:       st.Success.Render(ic.Done + " done"),
	}[t.Status]
	field(" ", "status", status)

	prio := st.Muted.Render("none")
	if t.Priority > domain.PriorityNone {
		prio = lipgloss.NewStyle().Foreground(st.Theme.Priority[t.Priority]).Bold(true).
			Render(ic.Priority + " " + t.Priority.String())
	}
	field(" ", "priority", prio)

	due := st.Muted.Render("none")
	if t.Due != nil {
		s := st.DueLater
		switch {
		case t.IsOverdue(now):
			s = st.DueOverdue
		case t.IsDueToday(now):
			s = st.DueToday
		}
		layout := "Mon 2 Jan"
		if t.Due.Year() != now.Year() {
			layout += " 2006"
		}
		if t.DueHasTime {
			layout += " · 15:04"
		}
		rel := st.Muted.Render(domain.Relative(*t.Due, t.DueHasTime, now))
		due = s.Render(t.Due.Format(layout))
		// Keep the relative part whole: wrap it under the date if needed.
		if 12+lipgloss.Width(due)+2+lipgloss.Width(rel) <= w {
			due += "  " + rel
		} else {
			due += "\n" + strings.Repeat(" ", 12) + rel
		}
	}
	field(ic.Due, "due", due)

	proj := st.Muted.Render("Inbox")
	if m.project.ID != "" {
		color := st.Project
		if m.project.Color != "" {
			color = lipgloss.NewStyle().Foreground(lipgloss.Color(m.project.Color))
		}
		proj = color.Render(strings.TrimSpace(m.project.Icon + " " + m.project.Name))
	}
	field(ic.Project, "project", proj)

	if t.Estimate > 0 || t.PomodorosDone > 0 || t.FocusTime > 0 {
		focus := st.Text.Render(fmt.Sprintf("%d", t.PomodorosDone))
		if t.Estimate > 0 {
			focus += st.Muted.Render(fmt.Sprintf(" of %d", t.Estimate))
		}
		focus += st.Muted.Render(" pomodoros")
		if t.FocusTime > 0 {
			focus += st.Muted.Render(" · ") + st.Text.Render(domain.FormatFocus(t.FocusTime))
		}
		field(ic.Tomato, "focus", focus)
	}

	if len(t.Tags) > 0 {
		chips := make([]string, len(t.Tags))
		for i, tag := range t.Tags {
			chips[i] = st.Tag.Render("#" + tag)
		}
		field(ic.Tag, "tags", strings.Join(chips, " "))
	}

	if len(m.subtasks) > 0 {
		b.WriteString("\n")
		done := 0
		for _, s := range m.subtasks {
			if s.IsDone() {
				done++
			}
		}
		b.WriteString(st.ModalTitle.Render(fmt.Sprintf("Subtasks %d/%d", done, len(m.subtasks))) + "  ")
		b.WriteString(progressBar(done, len(m.subtasks), 12, st.Theme.Success, st.Theme.Surface))
		b.WriteString("\n")
		for _, s := range m.subtasks {
			mark, ts := st.Muted.Render(ic.Todo), st.Text
			switch s.Status {
			case domain.StatusDone:
				mark, ts = st.Success.Render(ic.Done), st.DoneTitle
			case domain.StatusInProgress:
				mark = st.Warning.Render(ic.InProgress)
			}
			b.WriteString(" " + mark + " " + ts.Render(truncate(s.Title, w-5)) + "\n")
		}
	}

	b.WriteString("\n" + st.Muted.Render(strings.Repeat("─", w)) + "\n")
	if strings.TrimSpace(t.Description) == "" {
		b.WriteString(st.Empty.Render("No notes. Press " + m.ctx.Keys.Get(common.ActEdit).Help().Key + " to add some."))
		b.WriteString("\n")
	} else {
		b.WriteString(m.markdown(t.Description, w))
	}

	b.WriteString("\n\n" + st.Muted.Render("created "+t.CreatedAt.Format("2 Jan 2006 15:04")))
	if t.CompletedAt != nil {
		b.WriteString(st.Muted.Render(" · completed " + t.CompletedAt.Format("2 Jan 15:04")))
	} else if !t.UpdatedAt.Equal(t.CreatedAt) {
		b.WriteString(st.Muted.Render(" · edited " + t.UpdatedAt.Format("2 Jan 15:04")))
	}
	b.WriteString("\n" + st.Muted.Render("id "+domain.ShortID(t.ID)))
	return b.String()
}

// markdown renders the description, falling back to plain text if glamour
// fails (it never should, but a broken note must not break the pane).
func (m *Model) markdown(src string, w int) string {
	name := m.ctx.Styles.Theme.Name
	if m.renderer == nil || m.rendererWidth != w || m.rendererTheme != name {
		r, err := glamour.NewTermRenderer(
			glamour.WithStyles(glamourStyle(m.ctx)),
			glamour.WithWordWrap(w),
			glamour.WithEmoji(),
		)
		if err != nil {
			return lipgloss.NewStyle().Width(w).Render(src)
		}
		m.renderer, m.rendererWidth, m.rendererTheme = r, w, name
	}
	out, err := m.renderer.Render(src)
	if err != nil {
		return lipgloss.NewStyle().Width(w).Render(src)
	}
	return strings.Trim(out, "\n")
}

// glamourStyle adapts glamour's built-in style to the active theme.
func glamourStyle(ctx *common.Context) gansi.StyleConfig {
	th := ctx.Styles.Theme
	s := styles.DarkStyleConfig
	if !th.Dark {
		s = styles.LightStyleConfig
	}
	hex := func(c color.Color) *string {
		r, g, b, _ := c.RGBA()
		v := fmt.Sprintf("#%02x%02x%02x", r>>8, g>>8, b>>8)
		return &v
	}
	zero := uint(0)
	empty := ""
	t := true
	s.Document.Margin = &zero
	s.Document.BlockPrefix, s.Document.BlockSuffix = "", ""
	s.Document.Color = hex(th.Text)
	s.Heading.Color = hex(th.Accent)
	s.H1.Color, s.H1.BackgroundColor, s.H1.Bold = hex(th.Base), hex(th.Accent), &t
	s.H6.Color = hex(th.Subtext)
	// Headings are distinguished by color and weight, not "## " prefixes.
	for _, h := range []*gansi.StyleBlock{&s.H2, &s.H3, &s.H4, &s.H5, &s.H6} {
		h.Prefix = ""
	}
	s.Link.Color = hex(th.Info)
	s.LinkText.Color = hex(th.Info)
	s.Code.Color = hex(th.Warning)
	s.Code.BackgroundColor = hex(th.Surface)
	s.CodeBlock.Margin = &zero
	s.BlockQuote.Color = hex(th.Subtext)
	s.HorizontalRule.Color = hex(th.Muted)
	s.Enumeration.Color = hex(th.Accent)
	s.Item.BlockPrefix = "• "
	s.Task.Ticked, s.Task.Unticked = "[✓] ", "[ ] "
	s.Paragraph.BlockPrefix, s.Paragraph.BlockSuffix = empty, empty
	return s
}

func progressBar(done, total, w int, full, empty color.Color) string {
	n := 0
	if total > 0 {
		n = done * w / total
	}
	return lipgloss.NewStyle().Foreground(full).Render(strings.Repeat("━", n)) +
		lipgloss.NewStyle().Foreground(empty).Render(strings.Repeat("━", w-n))
}

func truncate(s string, w int) string {
	if w <= 1 {
		return ""
	}
	if lipgloss.Width(s) <= w {
		return s
	}
	r := []rune(s)
	for len(r) > 0 && lipgloss.Width(string(r))+1 > w {
		r = r[:len(r)-1]
	}
	return string(r) + "…"
}
