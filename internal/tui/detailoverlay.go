package tui

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/mahm0udnasr/tasker/internal/tui/common"
	"github.com/mahm0udnasr/tasker/internal/tui/detail"
)

// detailOverlay shows the detail pane as a modal on terminals too narrow for
// a third column. It owns a copy of the detail model sized to the modal.
type detailOverlay struct {
	ctx *common.Context
	d   detail.Model
}

func newDetailOverlay(ctx *common.Context, d detail.Model) *detailOverlay {
	o := &detailOverlay{ctx: ctx, d: d}
	o.resize()
	o.d.SetFocused(true)
	return o
}

func (o *detailOverlay) resize() {
	o.d.SetSize(min(o.ctx.Width-4, 72), max(o.ctx.Height-4, 6))
}

// Update implements common.Overlay.
func (o *detailOverlay) Update(msg tea.Msg) (common.Overlay, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		o.resize()
	case tea.KeyPressMsg:
		if msg.String() == "esc" || msg.String() == "q" || o.ctx.Keys.Is(msg, common.ActDetail) {
			return o, common.CloseOverlay
		}
		var cmd tea.Cmd
		o.d, cmd = o.d.Update(msg)
		return o, cmd
	}
	return o, nil
}

// View implements common.Overlay.
func (o *detailOverlay) View() string {
	return lipgloss.JoinVertical(lipgloss.Left, o.d.View(),
		o.ctx.Styles.KeyDesc.Render(" j/k scroll · esc close"))
}
