package overlay

import (
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/mahm0udnasr/tasker/internal/tui/common"
)

// Prompt is a one-line input with a live preview, e.g. for due dates.
type Prompt struct {
	ctx      *common.Context
	title    string
	input    textinput.Model
	preview  func(string) (text string, ok bool)
	onSubmit func(string) tea.Msg
	help     string
}

// PromptOptions configures a Prompt.
type PromptOptions struct {
	Title       string
	Value       string
	Placeholder string
	Suggestions []string
	Help        string
	// Preview renders feedback for the current value; ok=false blocks submit.
	Preview func(string) (string, bool)
	// OnSubmit builds the message delivered on Enter.
	OnSubmit func(string) tea.Msg
}

// NewPrompt creates a focused prompt.
func NewPrompt(ctx *common.Context, o PromptOptions) (*Prompt, tea.Cmd) {
	in := newInput(ctx, o.Placeholder, 40)
	in.SetValue(o.Value)
	in.CursorEnd()
	if len(o.Suggestions) > 0 {
		in.ShowSuggestions = true
		in.SetSuggestions(o.Suggestions)
	}
	p := &Prompt{ctx: ctx, title: o.Title, input: in, preview: o.Preview, onSubmit: o.OnSubmit, help: o.Help}
	return p, p.input.Focus()
}

// Update implements common.Overlay.
func (p *Prompt) Update(msg tea.Msg) (common.Overlay, tea.Cmd) {
	if km, ok := msg.(tea.KeyPressMsg); ok {
		switch {
		case isEsc(km):
			return p, common.CloseOverlay
		case km.String() == "enter":
			if p.preview != nil {
				if _, ok := p.preview(p.input.Value()); !ok {
					return p, nil
				}
			}
			return p, closeWith(p.onSubmit(p.input.Value()))
		}
	}
	var cmd tea.Cmd
	p.input, cmd = p.input.Update(msg)
	return p, cmd
}

// View implements common.Overlay.
func (p *Prompt) View() string {
	st := p.ctx.Styles
	lines := []string{st.ModalTitle.Render(p.title), "", p.input.View()}
	if p.preview != nil {
		text, ok := p.preview(p.input.Value())
		style := st.Success
		if !ok {
			style = st.Danger
		}
		lines = append(lines, "", style.Render(text))
	}
	help := p.help
	if help == "" {
		help = "enter to save · esc to cancel"
	}
	lines = append(lines, "", st.KeyDesc.Render(help))
	return st.Modal.Width(50).Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
}
