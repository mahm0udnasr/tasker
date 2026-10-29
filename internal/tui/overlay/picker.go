package overlay

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/sahilm/fuzzy"

	"github.com/mahm0udnasr/tasker/internal/tui/common"
)

// PickerItem is one choice.
type PickerItem struct {
	Icon  string
	Label string
	Hint  string // dimmed text on the right (e.g. a key binding or count)
	Value any
}

// Picker is a fuzzy-filtered list. It backs "move to project" and the
// command palette.
type Picker struct {
	ctx       *common.Context
	title     string
	input     textinput.Model
	items     []PickerItem
	matches   []match
	cursor    int
	maxRows   int
	onPick    func(PickerItem) tea.Msg
	createFmt string // e.g. `Create project "%s"`; "" disables creating
}

type match struct {
	item    PickerItem
	indexes []int // matched rune positions for highlighting
	create  bool
}

// PickerOptions configures a Picker.
type PickerOptions struct {
	Title       string
	Placeholder string
	Items       []PickerItem
	// Current preselects the item with this label.
	Current string
	// CreateFormat enables a "create" entry for unmatched input; the picked
	// item then has Value == nil and Label == the typed text.
	CreateFormat string
	OnPick       func(PickerItem) tea.Msg
}

// NewPicker creates a focused picker.
func NewPicker(ctx *common.Context, o PickerOptions) (*Picker, tea.Cmd) {
	p := &Picker{
		ctx: ctx, title: o.Title, items: o.Items, onPick: o.OnPick,
		createFmt: o.CreateFormat, maxRows: 10,
		input: newInput(ctx, o.Placeholder, 44),
	}
	p.filter()
	for i, m := range p.matches {
		if m.item.Label == o.Current {
			p.cursor = i
		}
	}
	return p, p.input.Focus()
}

func (p *Picker) filter() {
	q := strings.TrimSpace(p.input.Value())
	p.matches = p.matches[:0]
	if q == "" {
		for _, it := range p.items {
			p.matches = append(p.matches, match{item: it})
		}
	} else {
		labels := make([]string, len(p.items))
		for i, it := range p.items {
			labels[i] = it.Label
		}
		exact := false
		for _, r := range fuzzy.Find(q, labels) {
			p.matches = append(p.matches, match{item: p.items[r.Index], indexes: r.MatchedIndexes})
			exact = exact || strings.EqualFold(p.items[r.Index].Label, q)
		}
		if p.createFmt != "" && !exact {
			p.matches = append(p.matches, match{item: PickerItem{Label: q}, create: true})
		}
	}
	p.cursor = max(0, min(p.cursor, len(p.matches)-1))
}

// Update implements common.Overlay.
func (p *Picker) Update(msg tea.Msg) (common.Overlay, tea.Cmd) {
	if km, ok := msg.(tea.KeyPressMsg); ok {
		switch km.String() {
		case "esc":
			return p, common.CloseOverlay
		case "enter":
			if len(p.matches) == 0 {
				return p, nil
			}
			m := p.matches[p.cursor]
			return p, closeWith(p.onPick(m.item))
		case "up", "ctrl+p", "ctrl+k", "shift+tab":
			p.cursor = max(p.cursor-1, 0)
			return p, nil
		case "down", "ctrl+n", "ctrl+j", "tab":
			p.cursor = min(p.cursor+1, len(p.matches)-1)
			return p, nil
		}
	}
	prev := p.input.Value()
	var cmd tea.Cmd
	p.input, cmd = p.input.Update(msg)
	if p.input.Value() != prev {
		p.cursor = 0
		p.filter()
	}
	return p, cmd
}

// View implements common.Overlay.
func (p *Picker) View() string {
	st := p.ctx.Styles
	th := st.Theme
	const w = 48
	lines := []string{st.ModalTitle.Render(p.title), "", p.input.View(), ""}

	start := max(0, min(p.cursor-p.maxRows/2, len(p.matches)-p.maxRows))
	for i := start; i < len(p.matches) && i < start+p.maxRows; i++ {
		lines = append(lines, p.renderMatch(p.matches[i], i == p.cursor, w))
	}
	if len(p.matches) == 0 {
		lines = append(lines, st.Empty.Render("  no matches"))
	}
	if n := len(p.matches); n > p.maxRows {
		lines = append(lines, st.Muted.Render(fmt.Sprintf("  %d/%d", p.cursor+1, n)))
	}
	lines = append(lines, "", st.KeyDesc.Render("↑/↓ move · enter select · esc cancel"))
	return st.Modal.Width(w + 6).BorderForeground(th.Accent).Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
}

func (p *Picker) renderMatch(m match, selected bool, w int) string {
	st := p.ctx.Styles
	th := st.Theme
	base := lipgloss.NewStyle().Foreground(th.Subtext)
	hl := lipgloss.NewStyle().Foreground(th.Accent).Bold(true)
	if selected {
		base = base.Foreground(th.Text).Background(th.Surface)
		hl = hl.Background(th.Surface)
	}

	var label strings.Builder
	if m.create {
		label.WriteString(base.Render(fmt.Sprintf(p.createFmt, m.item.Label)))
	} else {
		matched := map[int]bool{}
		for _, i := range m.indexes {
			matched[i] = true
		}
		// fuzzy reports byte offsets; walk the string by byte index.
		for i, r := range m.item.Label {
			if matched[i] {
				label.WriteString(hl.Render(string(r)))
			} else {
				label.WriteString(base.Render(string(r)))
			}
		}
	}

	icon := m.item.Icon
	if m.create {
		icon = "+"
	}
	prefix := base.Render("  ")
	if selected {
		prefix = lipgloss.NewStyle().Foreground(th.Accent).Background(th.Surface).Render(p.ctx.Icons.Selected + " ")
	}
	left := prefix + base.Render(icon+" ") + label.String()
	hint := base.Foreground(th.Muted).Render(m.item.Hint)
	gap := max(w-lipgloss.Width(left)-lipgloss.Width(hint), 1)
	return left + base.Render(strings.Repeat(" ", gap)) + hint
}
