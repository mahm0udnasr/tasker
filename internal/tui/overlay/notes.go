package overlay

import (
	"os"
	"os/exec"
	"strings"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/mahm0udnasr/tasker/internal/tui/common"
)

// Notes is a multi-line markdown editor for a task's notes.
type Notes struct {
	ctx      *common.Context
	title    string
	input    textarea.Model
	original string
	onSubmit func(string) tea.Msg
}

// notesEditedMsg carries the text back from the external editor.
type notesEditedMsg struct {
	text string
	err  error
}

// NewNotes creates a focused editor. onSubmit receives the trimmed text, and
// only runs if it changed.
func NewNotes(ctx *common.Context, title, value string, onSubmit func(string) tea.Msg) (*Notes, tea.Cmd) {
	th := ctx.Styles.Theme
	in := textarea.New()
	in.ShowLineNumbers = false
	in.Prompt = ""
	in.Placeholder = "Markdown supported"
	in.MaxHeight = 0 // the default of 99 lines would cut longer notes on load
	s := textarea.DefaultStyles(th.Dark)
	s.Focused.Text = ctx.Styles.Text
	s.Focused.CursorLine = ctx.Styles.Text
	s.Focused.Placeholder = ctx.Styles.Muted
	s.Blurred = s.Focused
	s.Cursor.Color = th.Accent
	in.SetStyles(s)
	in.SetValue(value)
	n := &Notes{ctx: ctx, title: title, input: in, original: strings.TrimSpace(value), onSubmit: onSubmit}
	n.resize()
	return n, n.input.Focus()
}

func (n *Notes) contentWidth() int { return min(max(n.ctx.Width-8, 20), 70) }

func (n *Notes) resize() {
	n.input.SetWidth(n.contentWidth())
	n.input.SetHeight(min(max(n.ctx.Height-8, 3), 20))
}

// Update implements common.Overlay.
func (n *Notes) Update(msg tea.Msg) (common.Overlay, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		n.resize()
		return n, nil
	case notesEditedMsg:
		if msg.err != nil {
			return n, common.ErrorCmd(msg.err)
		}
		n.input.SetValue(strings.TrimRight(msg.text, "\r\n"))
		return n, nil
	case tea.KeyPressMsg:
		switch {
		case isEsc(msg):
			return n, common.CloseOverlay
		case msg.String() == "ctrl+s":
			text := strings.TrimSpace(n.input.Value())
			if text == n.original {
				return n, common.CloseOverlay
			}
			return n, closeWith(n.onSubmit(text))
		case msg.String() == "ctrl+e":
			return n, n.openEditor()
		}
	}
	var cmd tea.Cmd
	n.input, cmd = n.input.Update(msg)
	return n, cmd
}

// openEditor hands the text to $VISUAL or $EDITOR, keeping the current text
// if the editor can't be run.
func (n *Notes) openEditor() tea.Cmd {
	f, err := os.CreateTemp("", "tasker-notes-*.md")
	if err != nil {
		return common.ErrorCmd(err)
	}
	path := f.Name()
	_, err = f.WriteString(n.input.Value())
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		_ = os.Remove(path)
		return common.ErrorCmd(err)
	}
	editor := os.Getenv("VISUAL")
	if strings.TrimSpace(editor) == "" {
		editor = os.Getenv("EDITOR")
	}
	parts := strings.Fields(editor)
	if len(parts) == 0 {
		parts = []string{"nano"}
	}
	c := exec.Command(parts[0], append(parts[1:], path)...) //nolint:gosec,noctx // user-chosen editor
	return tea.ExecProcess(c, func(err error) tea.Msg {
		defer func() { _ = os.Remove(path) }()
		if err != nil {
			return notesEditedMsg{err: err}
		}
		data, err := os.ReadFile(path) //nolint:gosec // our own temp file
		return notesEditedMsg{text: string(data), err: err}
	})
}

// View implements common.Overlay.
func (n *Notes) View() string {
	st := n.ctx.Styles
	w := n.contentWidth()
	return st.Modal.Padding(0, 2).Width(w + 6).Render(lipgloss.JoinVertical(lipgloss.Left,
		st.ModalTitle.Render(ansi.Truncate(n.title, w, "…")),
		"",
		n.input.View(),
		"",
		st.KeyDesc.Render(ansi.Truncate("enter new line · ctrl+e $EDITOR · ctrl+s save · esc cancel", w, "…")),
	))
}
