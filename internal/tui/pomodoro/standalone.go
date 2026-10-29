package pomodoro

import (
	"context"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/mahm0udnasr/tasker/internal/domain"
	"github.com/mahm0udnasr/tasker/internal/store"
	"github.com/mahm0udnasr/tasker/internal/tui/common"
)

// standalone is the `tasker focus` program: just the focus view.
type standalone struct {
	m      *Model
	task   domain.Task
	resume bool // a session on this task is already running
	w, h   int
	status string
}

// RunStandalone runs a full-screen focus timer for task. If a session for
// the task is already running (e.g. started in the TUI) it is resumed.
// Quitting leaves a running session running; the TUI picks it up.
func RunStandalone(ctx *common.Context, s store.Store, opts Options, task domain.Task, resume bool) error {
	p := tea.NewProgram(&standalone{m: New(ctx, s, opts), task: task, resume: resume})
	_, err := p.Run()
	return err
}

func (a *standalone) Init() tea.Cmd {
	if a.resume {
		return a.m.Init()
	}
	return a.m.Start(a.task)
}

func (a *standalone) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.w, a.h = msg.Width, msg.Height
		a.m.ctx.Width, a.m.ctx.Height = msg.Width, msg.Height
		return a, nil
	case tea.KeyPressMsg:
		switch msg.String() {
		case "q", "esc", "ctrl+c":
			return a, tea.Quit
		}
		cmd, _ := HandleKey(a.m, msg, &a.task)
		return a, cmd
	case common.StatusMsg:
		a.status = msg.Text
		return a, nil
	case CompleteTaskMsg:
		return a, a.completeTask(msg.TaskID)
	case ChangedMsg:
		return a, nil
	}
	return a, a.m.Update(msg)
}

func (a *standalone) completeTask(id string) tea.Cmd {
	s, now := a.m.store, a.m.ctx.Now()
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		t, err := s.GetTask(ctx, id)
		if err != nil {
			return common.StatusMsg{Text: err.Error(), Error: true}
		}
		t.SetStatus(domain.StatusDone, now)
		if err := s.UpdateTask(ctx, &t); err != nil {
			return common.StatusMsg{Text: err.Error(), Error: true}
		}
		return tea.Quit()
	}
}

func (a *standalone) View() tea.View {
	th := a.m.ctx.Styles.Theme
	if a.w == 0 { // first frame, before the terminal size is known
		v := tea.NewView("")
		v.AltScreen = true
		return v
	}
	body := a.m.FocusView(a.w, max(a.h-1, 1))
	status := a.m.ctx.Styles.Muted.Render(" " + a.status + "  ·  q leaves the timer running")
	v := tea.NewView(body + "\n" + status)
	v.AltScreen = true
	v.BackgroundColor, v.ForegroundColor = th.Base, th.Text
	v.WindowTitle = "tasker · focus"
	v.ProgressBar = a.m.TerminalProgress()
	return v
}
