// Package form is the add/edit task modal, built on huh.
package form

import (
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/mahm0udnasr/tasker/internal/domain"
	"github.com/mahm0udnasr/tasker/internal/tui/common"
)

const width = 60

// Model wraps a huh.Form. Field values are bound to the heap-allocated
// draft, so they survive the value copies Bubble Tea makes.
type Model struct {
	ctx   *common.Context
	form  *huh.Form
	draft *common.TaskDraft
	title string
	err   string
}

// Options seeds the form.
type Options struct {
	Task       *domain.Task // nil for a new task
	ParentID   string       // for a new subtask
	ParentName string
	Project    string // prefilled project name (current scope or the task's)
	Due        string // prefilled due input for new tasks
	Tags       string // prefilled tags for new tasks
	Projects   []string
}

// New builds the form overlay.
func New(ctx *common.Context, o Options) (*Model, tea.Cmd) {
	d := &common.TaskDraft{ParentID: o.ParentID, Project: o.Project, Due: o.Due, Tags: o.Tags}
	title := "New task"
	switch {
	case o.Task != nil:
		t := o.Task
		title = "Edit task"
		d.ID, d.ParentID = t.ID, t.ParentID
		d.Title, d.Description, d.Priority = t.Title, t.Description, t.Priority
		d.Tags = strings.Join(t.Tags, ", ")
		if t.Estimate > 0 {
			d.Estimate = strconv.Itoa(t.Estimate)
		}
		if t.Due != nil {
			d.Due = t.Due.Format("2006-01-02")
			if t.DueHasTime {
				d.Due = t.Due.Format("2006-01-02 15:04")
			}
		}
	case o.ParentID != "":
		title = "New subtask of “" + o.ParentName + "”"
	}

	isSubtask := d.ParentID != ""
	fields := []huh.Field{
		huh.NewInput().Key("title").Title("Title").Placeholder("What needs doing?").
			Value(&d.Title).CharLimit(domain.MaxTitleLen).
			Validate(func(s string) error {
				if strings.TrimSpace(s) == "" {
					return domain.ErrEmptyTitle
				}
				return nil
			}),
		huh.NewText().Key("description").Title("Notes").Placeholder("Markdown supported").
			Lines(2).Value(&d.Description),
		huh.NewInput().Key("due").Title("Due").Placeholder(`"tomorrow 5pm", "fri", "2026-12-24"`).
			Value(&d.Due).
			DescriptionFunc(func() string { return duePreview(ctx, d.Due) }, &d.Due).
			Validate(func(s string) error {
				_, _, err := domain.ParseDue(s, ctx.Now())
				return err
			}),
		huh.NewSelect[domain.Priority]().Key("priority").Title("Priority").Inline(true).
			Options(priorityOptions()...).Value(&d.Priority),
	}
	if !isSubtask { // subtasks always live in their parent's project
		fields = append(fields, huh.NewInput().Key("project").Title("Project").
			Placeholder("Inbox").Suggestions(o.Projects).Value(&d.Project))
	}
	fields = append(fields, huh.NewInput().Key("tags").Title("Tags").
		Placeholder("comma, separated").Value(&d.Tags),
		huh.NewInput().Key("estimate").Title("Estimate").Placeholder("pomodoros, e.g. 4 (optional)").
			Value(&d.Estimate).CharLimit(2).Validate(func(s string) error {
			_, err := ParseEstimate(s)
			return err
		}))

	f := huh.NewForm(huh.NewGroup(fields...)).
		WithTheme(huhTheme(ctx)).
		WithWidth(width).
		WithShowHelp(false).
		WithShowErrors(true)
	f.SubmitCmd, f.CancelCmd = nil, nil

	return &Model{ctx: ctx, form: f, draft: d, title: title}, f.Init()
}

func priorityOptions() []huh.Option[domain.Priority] {
	ps := []domain.Priority{domain.PriorityNone, domain.PriorityLow, domain.PriorityMedium,
		domain.PriorityHigh, domain.PriorityUrgent}
	out := make([]huh.Option[domain.Priority], len(ps))
	for i, p := range ps {
		out[i] = huh.NewOption(p.String(), p)
	}
	return out
}

func duePreview(ctx *common.Context, s string) string {
	if strings.TrimSpace(s) == "" {
		return "no due date"
	}
	due, hasTime, err := domain.ParseDue(s, ctx.Now())
	if err != nil {
		return "✗ can't read that date"
	}
	if due == nil {
		return "no due date"
	}
	layout := "Mon 2 Jan 2006"
	if hasTime {
		layout += " · 15:04"
	}
	return "→ " + due.Format(layout)
}

// Update implements common.Overlay.
func (m *Model) Update(msg tea.Msg) (common.Overlay, tea.Cmd) {
	if km, ok := msg.(tea.KeyPressMsg); ok {
		switch km.String() {
		case "esc":
			return m, common.CloseOverlay
		case "ctrl+s":
			return m, m.submit()
		}
	}
	f, cmd := m.form.Update(msg)
	if hf, ok := f.(*huh.Form); ok {
		m.form = hf
	}
	switch m.form.State {
	case huh.StateCompleted:
		return m, m.submit()
	case huh.StateAborted:
		return m, common.CloseOverlay
	}
	return m, cmd
}

// submit validates and emits the draft. Validation is repeated here because
// ctrl+s can submit before huh has validated the focused field.
func (m *Model) submit() tea.Cmd {
	d := *m.draft
	d.Title = strings.TrimSpace(d.Title)
	if d.Title == "" {
		m.err = "Title is required"
		m.form.State = huh.StateNormal
		return nil
	}
	if _, _, err := domain.ParseDue(d.Due, m.ctx.Now()); err != nil {
		m.err = err.Error()
		m.form.State = huh.StateNormal
		return nil
	}
	if _, err := ParseEstimate(d.Estimate); err != nil {
		m.err = err.Error()
		m.form.State = huh.StateNormal
		return nil
	}
	return tea.Sequence(common.CloseOverlay, func() tea.Msg { return common.SaveDraftMsg{Draft: d} })
}

// View implements common.Overlay.
func (m *Model) View() string {
	st := m.ctx.Styles
	parts := []string{"", st.ModalTitle.Render(m.title), "", trimBlankLines(m.form.View())}
	if m.err != "" {
		parts = append(parts, st.Danger.Render(m.ctx.Icons.Cross+" "+m.err))
	}
	parts = append(parts, "", st.KeyDesc.Render("tab/shift+tab move · ctrl+s save · esc cancel"))
	// Vertical padding is dropped so the form fits 24-row terminals.
	return st.Modal.Padding(0, 2).Width(width + 6).Render(lipgloss.JoinVertical(lipgloss.Left, parts...))
}

// huhTheme maps tasker's palette onto huh's styles.
func huhTheme(ctx *common.Context) huh.Theme {
	return huh.ThemeFunc(func(isDark bool) *huh.Styles {
		th := ctx.Styles.Theme
		t := huh.ThemeBase(isDark)
		f := &t.Focused
		f.Base = f.Base.BorderForeground(th.Accent)
		f.Card = f.Base
		f.Title = lipgloss.NewStyle().Foreground(th.Accent).Bold(true)
		f.Description = lipgloss.NewStyle().Foreground(th.Muted)
		f.ErrorIndicator = lipgloss.NewStyle().Foreground(th.Danger).SetString(" *")
		f.ErrorMessage = lipgloss.NewStyle().Foreground(th.Danger)
		f.SelectSelector = lipgloss.NewStyle().Foreground(th.Accent).SetString("❯ ")
		f.NextIndicator = lipgloss.NewStyle().Foreground(th.Accent).MarginLeft(1).SetString("→")
		f.PrevIndicator = lipgloss.NewStyle().Foreground(th.Accent).MarginRight(1).SetString("←")
		f.Option = lipgloss.NewStyle().Foreground(th.Text)
		f.SelectedOption = lipgloss.NewStyle().Foreground(th.Accent)
		f.TextInput.Cursor = lipgloss.NewStyle().Foreground(th.Accent)
		f.TextInput.Placeholder = lipgloss.NewStyle().Foreground(th.Muted)
		f.TextInput.Prompt = lipgloss.NewStyle().Foreground(th.Accent)
		f.TextInput.Text = lipgloss.NewStyle().Foreground(th.Text)

		t.Blurred = *f
		b := &t.Blurred
		b.Base = b.Base.BorderStyle(lipgloss.HiddenBorder())
		b.Card = b.Base
		b.Title = lipgloss.NewStyle().Foreground(th.Subtext)
		b.TextInput.Text = lipgloss.NewStyle().Foreground(th.Subtext)
		b.NextIndicator = lipgloss.NewStyle()
		b.PrevIndicator = lipgloss.NewStyle()
		b.Option = lipgloss.NewStyle().Foreground(th.Subtext)
		t.FieldSeparator = lipgloss.NewStyle().SetString("\n")
		return t
	})
}

// ParseEstimate reads the estimate field: empty means none.
func ParseEstimate(s string) (int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 || n > domain.MaxEstimate {
		return 0, domain.ErrInvalidEstimate
	}
	return n, nil
}

// trimBlankLines drops the trailing empty lines huh pads its group with
// (it reserves room for the tallest possible field state).
func trimBlankLines(s string) string {
	lines := strings.Split(s, "\n")
	for len(lines) > 0 && strings.TrimSpace(ansi.Strip(lines[len(lines)-1])) == "" {
		lines = lines[:len(lines)-1]
	}
	return strings.Join(lines, "\n")
}
