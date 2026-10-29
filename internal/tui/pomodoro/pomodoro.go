// Package pomodoro runs the focus timer: focus → break → focus cycles tied
// to a task, persisted so a session survives restarts, with desktop
// notifications when a phase ends.
//
// The root model holds a *Model (shared across Bubble Tea's value copies,
// which is safe because Update runs on one goroutine) so the status bar,
// task list and focus overlay can all render the same live timer.
package pomodoro

import (
	"context"
	"errors"
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/mahm0udnasr/tasker/internal/domain"
	"github.com/mahm0udnasr/tasker/internal/notify"
	"github.com/mahm0udnasr/tasker/internal/store"
	"github.com/mahm0udnasr/tasker/internal/tui/common"
)

// Phase is what the timer is doing.
type Phase int

// Timer phases.
const (
	Idle Phase = iota
	Focus
	ShortBreak
	LongBreak
)

// Label names the phase for display.
func (p Phase) Label() string {
	switch p {
	case Focus:
		return "Focus"
	case ShortBreak:
		return "Short break"
	case LongBreak:
		return "Long break"
	}
	return "Idle"
}

// Options configures the controller.
type Options struct {
	Settings domain.PomodoroSettings
	Notify   bool // desktop notifications
	Bell     bool // terminal bell
}

// ChangedMsg tells the root that pomodoro counts changed (reload the list).
type ChangedMsg struct{}

// CompleteTaskMsg asks the root to mark the focused task done.
type CompleteTaskMsg struct{ TaskID string }

type (
	tickMsg   struct{ gen int }
	loadedMsg struct {
		session *domain.Pomodoro
		task    domain.Task
		err     error
	}
	savedMsg struct{ err error }
)

// Model is the timer controller.
type Model struct {
	ctx   *common.Context
	store store.Store
	opts  Options

	phase   Phase
	session domain.Pomodoro  // valid during Focus
	brk     domain.Countdown // valid during breaks
	task    domain.Task      // the focused task; kept across breaks
	cycle   int              // focus sessions completed since the last long break
	tickGen int

	// notifier is swappable so tests don't pop real notifications.
	notifier func(title, body string) error
}

// New creates an idle controller.
func New(ctx *common.Context, s store.Store, opts Options) *Model {
	return &Model{ctx: ctx, store: s, opts: opts, notifier: notify.Send}
}

// SetNotifier replaces the desktop notifier (tests).
func (m *Model) SetNotifier(f func(title, body string) error) { m.notifier = f }

// Init restores a session left running by a previous run (or by
// `tasker focus`).
func (m *Model) Init() tea.Cmd {
	s := m.store
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		p, err := s.ActivePomodoro(ctx)
		if errors.Is(err, store.ErrNotFound) {
			return loadedMsg{}
		}
		if err != nil {
			return loadedMsg{err: err}
		}
		t, err := s.GetTask(ctx, p.TaskID)
		return loadedMsg{session: &p, task: t, err: err}
	}
}

// Phase returns the current phase.
func (m *Model) Phase() Phase { return m.phase }

// Active reports whether a focus session or break is in progress.
func (m *Model) Active() bool { return m.phase != Idle }

// Task returns the task being focused on (also during its breaks).
func (m *Model) Task() domain.Task { return m.task }

// Paused reports whether the current phase is paused.
func (m *Model) Paused() bool {
	switch m.phase {
	case Focus:
		return m.session.IsPaused()
	case ShortBreak, LongBreak:
		return m.brk.IsPaused()
	}
	return false
}

// Remaining is the time left in the current phase.
func (m *Model) Remaining() time.Duration {
	switch m.phase {
	case Focus:
		return m.session.Remaining(m.ctx.Now())
	case ShortBreak, LongBreak:
		return m.brk.Remaining(m.ctx.Now())
	}
	return 0
}

// Progress is the elapsed fraction of the current phase.
func (m *Model) Progress() float64 {
	switch m.phase {
	case Focus:
		return m.session.Progress(m.ctx.Now())
	case ShortBreak, LongBreak:
		return m.brk.Progress(m.ctx.Now())
	}
	return 0
}

// Cycle returns completed sessions in this cycle and the cycle length.
func (m *Model) Cycle() (done, of int) { return m.cycle, m.opts.Settings.LongEvery }

// --- commands ----------------------------------------------------------------

// Toggle starts a focus session on task when idle, or pauses/resumes the
// running phase. task may be nil (then the last focused task is reused).
func (m *Model) Toggle(task *domain.Task) tea.Cmd {
	if m.phase != Idle {
		return m.togglePause()
	}
	switch {
	case task != nil:
		return m.Start(*task)
	case m.task.ID != "":
		return m.Start(m.task)
	}
	return common.ErrorCmd(errors.New("select a task to focus on"))
}

// Start begins a focus session on t, ending any break in progress.
func (m *Model) Start(t domain.Task) tea.Cmd {
	if m.phase == Focus {
		return common.ErrorCmd(fmt.Errorf("already focusing on “%s” — stop it first", m.task.Title))
	}
	if t.IsDone() {
		return common.ErrorCmd(errors.New("that task is already done"))
	}
	now := m.ctx.Now()
	m.phase, m.task = Focus, t
	m.session = domain.NewPomodoro(t.ID, m.opts.Settings.Focus, now)
	p := m.session
	s := m.store
	save := func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return savedMsg{err: s.CreatePomodoro(ctx, &p)}
	}
	return tea.Batch(save, m.tick(),
		common.Status(fmt.Sprintf("%s Focus on “%s” — %s", m.ctx.Icons.Tomato, t.Title,
			domain.FormatClock(m.opts.Settings.Focus))))
}

func (m *Model) togglePause() tea.Cmd {
	now := m.ctx.Now()
	var c *domain.Countdown
	if m.phase == Focus {
		c = &m.session.Countdown
	} else {
		c = &m.brk
	}
	var text string
	var cmds []tea.Cmd
	if c.IsPaused() {
		c.Resume(now)
		text = "Resumed"
		cmds = append(cmds, m.tick())
	} else {
		c.Pause(now)
		text = "Paused"
		m.tickGen++ // stop the ticker
	}
	if m.phase == Focus {
		m.session.UpdatedAt = now
		cmds = append(cmds, m.persist())
	}
	return tea.Batch(append(cmds, common.Status(text+" · "+domain.FormatClock(m.Remaining())+" left"))...)
}

// Stop ends the current phase: a focus session is recorded as interrupted
// (its focus time still counts), a break is simply dropped.
func (m *Model) Stop() tea.Cmd {
	now := m.ctx.Now()
	m.tickGen++
	switch m.phase {
	case Focus:
		m.session.Finish(now)
		m.phase = Idle
		text := fmt.Sprintf("Stopped · %s of focus on “%s”", domain.FormatFocus(m.session.Focused(now)), m.task.Title)
		if m.session.Completed {
			text = "Focus session complete"
		}
		return tea.Batch(m.persist(), changed, common.Status(text))
	case ShortBreak, LongBreak:
		m.phase = Idle
		return common.Status("Break ended")
	}
	return common.Status("No timer running")
}

// Skip jumps to the next phase: an unfinished focus session ends early and
// its break starts; a break ends (and the next focus starts if configured).
func (m *Model) Skip() tea.Cmd {
	switch m.phase {
	case Focus:
		m.session.Finish(m.ctx.Now())
		return tea.Batch(m.persist(), changed, m.startBreak(false))
	case ShortBreak, LongBreak:
		return m.endBreak("Break skipped")
	}
	return common.Status("No timer running")
}

// CompleteTask stops the session and asks the root to mark the task done.
func (m *Model) CompleteTask() tea.Cmd {
	if m.task.ID == "" {
		return nil
	}
	id := m.task.ID
	var cmd tea.Cmd
	if m.phase == Focus {
		cmd = m.Stop()
	}
	return tea.Batch(cmd, func() tea.Msg { return CompleteTaskMsg{TaskID: id} })
}

func changed() tea.Msg { return ChangedMsg{} }

func (m *Model) tick() tea.Cmd {
	m.tickGen++
	gen := m.tickGen
	// Tick on the next whole second of the countdown so the display
	// changes exactly when the shown second does.
	wait := m.Remaining() % time.Second
	if wait <= 0 {
		wait = time.Second
	}
	return common.Tick(wait, func(time.Time) tea.Msg { return tickMsg{gen: gen} })
}

func (m *Model) persist() tea.Cmd {
	p := m.session
	s := m.store
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return savedMsg{err: s.UpdatePomodoro(ctx, &p)}
	}
}

// Update handles the controller's own messages; others are ignored.
func (m *Model) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tickMsg:
		if msg.gen != m.tickGen || m.Paused() {
			return nil
		}
		now := m.ctx.Now()
		switch m.phase {
		case Focus:
			if m.session.Done(now) {
				return m.completeFocus()
			}
		case ShortBreak, LongBreak:
			if m.brk.Done(now) {
				return m.endBreak("Break over")
			}
		default:
			return nil
		}
		return m.tick()

	case loadedMsg:
		if msg.err != nil {
			return common.ErrorCmd(fmt.Errorf("restore focus session: %w", msg.err))
		}
		if msg.session == nil {
			return nil
		}
		m.session, m.task, m.phase = *msg.session, msg.task, Focus
		if m.session.Done(m.ctx.Now()) {
			// It ran out while tasker was closed: record it as completed.
			m.session.Finish(m.ctx.Now())
			m.phase = Idle
			m.cycle++
			return tea.Batch(m.persist(), changed,
				common.Status(fmt.Sprintf("%s A focus session on “%s” finished while tasker was closed",
					m.ctx.Icons.Tomato, m.task.Title)))
		}
		if m.session.IsPaused() {
			return common.Status("Focus session on “" + m.task.Title + "” is paused · P to resume")
		}
		return m.tick()

	case savedMsg:
		if msg.err != nil {
			if errors.Is(msg.err, store.ErrSessionRunning) {
				m.phase = Idle
				m.tickGen++
				return common.ErrorCmd(errors.New("another focus session is running (in another tasker?)"))
			}
			return common.ErrorCmd(fmt.Errorf("save focus session: %w", msg.err))
		}
	}
	return nil
}

func (m *Model) completeFocus() tea.Cmd {
	m.session.Finish(m.ctx.Now())
	m.cycle++
	done, of := m.cycle, m.opts.Settings.LongEvery
	cmds := []tea.Cmd{m.persist(), changed}
	cmds = append(cmds, m.alert(m.ctx.Icons.Tomato+" Focus complete",
		fmt.Sprintf("“%s” · %d of %d this cycle. Time for a break.", m.task.Title, done, of)))
	if m.opts.Settings.AutoBreaks {
		cmds = append(cmds, m.startBreak(true))
	} else {
		m.phase = Idle
		cmds = append(cmds, common.Status("Focus complete · P to start another"))
	}
	return tea.Batch(cmds...)
}

// startBreak begins the break after a focus session. counted is false for
// a skipped (unfinished) session, which always earns a short break.
func (m *Model) startBreak(counted bool) tea.Cmd {
	long, d := m.opts.Settings.BreakAfter(m.cycle)
	if !counted {
		long, d = false, m.opts.Settings.ShortBreak
	}
	m.phase = ShortBreak
	if long {
		m.phase = LongBreak
	}
	m.brk = domain.NewCountdown(d, m.ctx.Now())
	return tea.Batch(m.tick(), common.Status(fmt.Sprintf("%s %s — %s", m.ctx.Icons.Coffee,
		m.phase.Label(), domain.FormatClock(d))))
}

func (m *Model) endBreak(reason string) tea.Cmd {
	if m.phase == LongBreak {
		m.cycle = 0
	}
	m.phase = Idle
	m.tickGen++
	cmds := []tea.Cmd{}
	if reason == "Break over" {
		cmds = append(cmds, m.alert(m.ctx.Icons.Coffee+" Break over", "Back to “"+m.task.Title+"”?"))
	}
	if m.opts.Settings.AutoFocus && m.task.ID != "" {
		return tea.Batch(append(cmds, m.Start(m.task))...)
	}
	return tea.Batch(append(cmds, common.Status(reason+" · P to focus on “"+m.task.Title+"”"))...)
}

// alert rings the bell and sends a desktop notification, as configured.
func (m *Model) alert(title, body string) tea.Cmd {
	var cmds []tea.Cmd
	if m.opts.Bell {
		cmds = append(cmds, tea.Raw("\a"))
	}
	if m.opts.Notify && m.notifier != nil {
		send := m.notifier
		cmds = append(cmds, func() tea.Msg {
			_ = send(title, body) // best effort: the status bar shows it anyway
			return nil
		})
	}
	return tea.Batch(cmds...)
}

// TerminalProgress reports the phase to terminals that show a native
// progress indicator (OSC 9;4), or nil when idle.
func (m *Model) TerminalProgress() *tea.ProgressBar {
	if m.phase == Idle {
		return nil
	}
	state := tea.ProgressBarDefault
	if m.Paused() {
		state = tea.ProgressBarWarning
	}
	return tea.NewProgressBar(state, int(m.Progress()*100))
}
