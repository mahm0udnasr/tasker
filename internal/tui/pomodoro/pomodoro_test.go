package pomodoro

import (
	"context"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/mahm0udnasr/tasker/internal/domain"
	"github.com/mahm0udnasr/tasker/internal/store/sqlite"
	"github.com/mahm0udnasr/tasker/internal/theme"
	"github.com/mahm0udnasr/tasker/internal/tui/common"
)

type fixture struct {
	t        *testing.T
	m        *Model
	now      time.Time
	store    *sqlite.Store
	task     domain.Task
	notes    []string // desktop notifications sent
	statuses []string // status-bar messages
	changed  int      // ChangedMsg count
}

func newFixture(t *testing.T, mod func(*domain.PomodoroSettings)) *fixture {
	t.Helper()
	s, err := sqlite.Open(context.Background(), filepath.Join(t.TempDir(), "p.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	orig := common.Tick // ticks are delivered explicitly by advance()
	common.Tick = func(time.Duration, func(time.Time) tea.Msg) tea.Cmd { return nil }
	t.Cleanup(func() { common.Tick = orig })
	f := &fixture{t: t, store: s, now: time.Date(2026, 10, 29, 9, 0, 0, 0, time.Local)}
	f.task = domain.NewTask("Write thesis", f.now)
	if err := s.CreateTask(context.Background(), &f.task); err != nil {
		t.Fatal(err)
	}
	settings := domain.DefaultPomodoroSettings()
	if mod != nil {
		mod(&settings)
	}
	ctx := &common.Context{
		Styles: theme.NewStyles(theme.Get("")), Icons: theme.NerdIcons,
		Keys: common.NewKeyMap(nil), Now: func() time.Time { return f.now },
	}
	f.m = New(ctx, s, Options{Settings: settings, Notify: true})
	f.m.SetNotifier(func(title, _ string) error {
		f.notes = append(f.notes, title)
		return nil
	})
	return f
}

var cmdSlice = reflect.TypeOf([]tea.Cmd(nil))

// run executes cmd synchronously, waiting for it however long it takes,
// and routes results back into the model. Timers are no-ops here (see
// newFixture); tests deliver ticks explicitly via advance.
func (f *fixture) run(cmd tea.Cmd) {
	if cmd == nil {
		return
	}
	done := make(chan tea.Msg, 1)
	go func() { done <- cmd() }()
	var msg tea.Msg
	select {
	case msg = <-done:
	case <-time.After(10 * time.Second):
		f.t.Fatalf("command did not finish within 10s")
	}
	switch msg := msg.(type) {
	case nil:
	case tea.BatchMsg:
		for _, c := range msg {
			f.run(c)
		}
	case common.StatusMsg:
		f.statuses = append(f.statuses, msg.Text)
	case ChangedMsg:
		f.changed++
	default:
		if v := reflect.ValueOf(msg); v.Kind() == reflect.Slice && v.Type().ConvertibleTo(cmdSlice) {
			for _, c := range v.Convert(cmdSlice).Interface().([]tea.Cmd) {
				f.run(c)
			}
			return
		}
		f.run(f.m.Update(msg))
	}
}

// advance moves the clock and delivers the pending tick.
func (f *fixture) advance(d time.Duration) {
	f.now = f.now.Add(d)
	f.run(f.m.Update(tickMsg{gen: f.m.tickGen}))
}

func (f *fixture) lastStatus() string {
	if len(f.statuses) == 0 {
		return ""
	}
	return f.statuses[len(f.statuses)-1]
}

func (f *fixture) reloadTask() domain.Task {
	t, err := f.store.GetTask(context.Background(), f.task.ID)
	if err != nil {
		f.t.Fatal(err)
	}
	return t
}

func TestFullFocusCycleWithLongBreak(t *testing.T) {
	f := newFixture(t, func(s *domain.PomodoroSettings) { s.LongEvery = 2 })
	f.run(f.m.Toggle(&f.task))
	if f.m.Phase() != Focus || !strings.Contains(f.lastStatus(), "Focus on “Write thesis” — 25:00") {
		t.Fatalf("phase=%v status=%q", f.m.Phase(), f.lastStatus())
	}
	if _, err := f.store.ActivePomodoro(context.Background()); err != nil {
		t.Fatalf("session not persisted: %v", err)
	}

	f.advance(10 * time.Minute)
	if f.m.Phase() != Focus || f.m.Remaining() != 15*time.Minute {
		t.Fatalf("mid-session: %v %v", f.m.Phase(), f.m.Remaining())
	}

	// Session 1 completes → short break.
	f.advance(15 * time.Minute)
	if f.m.Phase() != ShortBreak || len(f.notes) != 1 || f.changed != 1 {
		t.Fatalf("after focus 1: phase=%v notes=%v changed=%d", f.m.Phase(), f.notes, f.changed)
	}
	if got := f.reloadTask(); got.PomodorosDone != 1 || got.FocusTime != 25*time.Minute {
		t.Errorf("task after 1: done=%d focus=%v", got.PomodorosDone, got.FocusTime)
	}

	// Break ends → idle (AutoFocus off), waiting for P.
	f.advance(5 * time.Minute)
	if f.m.Phase() != Idle || !strings.Contains(f.lastStatus(), "P to focus on “Write thesis”") {
		t.Fatalf("after break: %v %q", f.m.Phase(), f.lastStatus())
	}

	// Session 2 (Toggle with no selection reuses the last task) → long break.
	f.run(f.m.Toggle(nil))
	f.advance(25 * time.Minute)
	if f.m.Phase() != LongBreak {
		t.Fatalf("second break should be long, got %v", f.m.Phase())
	}
	if done, of := f.m.Cycle(); done != 2 || of != 2 {
		t.Errorf("cycle = %d/%d", done, of)
	}
	f.advance(15 * time.Minute)
	if done, _ := f.m.Cycle(); done != 0 || f.m.Phase() != Idle {
		t.Errorf("cycle should reset after a long break: %d, %v", done, f.m.Phase())
	}
	if len(f.notes) != 4 { // focus, break, focus, break
		t.Errorf("notifications = %v", f.notes)
	}
}

func TestPauseFreezesTimeAndPersists(t *testing.T) {
	f := newFixture(t, nil)
	f.run(f.m.Start(f.task))
	f.advance(5 * time.Minute)
	f.run(f.m.Toggle(nil)) // pause
	if !f.m.Paused() {
		t.Fatal("not paused")
	}
	f.now = f.now.Add(time.Hour) // a long lunch
	if f.m.Remaining() != 20*time.Minute {
		t.Errorf("remaining while paused = %v", f.m.Remaining())
	}
	active, _ := f.store.ActivePomodoro(context.Background())
	if !active.IsPaused() {
		t.Error("pause not persisted")
	}
	f.run(f.m.Toggle(nil)) // resume
	f.advance(20 * time.Minute)
	if f.m.Phase() != ShortBreak {
		t.Errorf("should complete 20m after resuming, phase=%v", f.m.Phase())
	}
}

func TestStopRecordsInterruptedFocus(t *testing.T) {
	f := newFixture(t, nil)
	f.run(f.m.Start(f.task))
	f.advance(12 * time.Minute)
	f.run(f.m.Stop())
	if f.m.Phase() != Idle || !strings.Contains(f.lastStatus(), "Stopped · 12m of focus") {
		t.Fatalf("phase=%v status=%q", f.m.Phase(), f.lastStatus())
	}
	got := f.reloadTask()
	if got.PomodorosDone != 0 || got.FocusTime != 12*time.Minute {
		t.Errorf("interrupted session: done=%d focus=%v", got.PomodorosDone, got.FocusTime)
	}
	// A stale tick from before the stop must do nothing.
	f.run(f.m.Update(tickMsg{gen: f.m.tickGen - 1}))
	if f.m.Phase() != Idle {
		t.Error("stale tick revived the timer")
	}
}

func TestSkipAndAutoFocus(t *testing.T) {
	f := newFixture(t, func(s *domain.PomodoroSettings) { s.AutoFocus = true })
	f.run(f.m.Start(f.task))
	f.advance(3 * time.Minute)
	f.run(f.m.Skip()) // unfinished focus → short break, not counted
	if f.m.Phase() != ShortBreak {
		t.Fatalf("phase = %v", f.m.Phase())
	}
	if done, _ := f.m.Cycle(); done != 0 {
		t.Errorf("skipped session counted: %d", done)
	}
	f.run(f.m.Skip()) // skip the break → auto-starts the next focus
	if f.m.Phase() != Focus || f.m.Remaining() != 25*time.Minute {
		t.Errorf("auto focus: %v %v", f.m.Phase(), f.m.Remaining())
	}
}

func TestStartGuards(t *testing.T) {
	f := newFixture(t, nil)
	f.run(f.m.Toggle(nil))
	if f.m.Phase() != Idle || !strings.Contains(f.lastStatus(), "select a task") {
		t.Errorf("toggle with nothing selected: %q", f.lastStatus())
	}
	f.run(f.m.Start(f.task))
	f.run(f.m.Start(f.task))
	if !strings.Contains(f.lastStatus(), "already focusing") {
		t.Errorf("double start: %q", f.lastStatus())
	}
	done := f.task
	done.SetStatus(domain.StatusDone, f.now)
	f.run(f.m.Stop())
	f.run(f.m.Start(done))
	if !strings.Contains(f.lastStatus(), "already done") {
		t.Errorf("start on done task: %q", f.lastStatus())
	}
}

func TestRestoreRunningSession(t *testing.T) {
	f := newFixture(t, nil)
	f.run(f.m.Start(f.task))
	f.advance(10 * time.Minute)

	// A new controller (tasker restarted) picks the session up.
	g := newFixture(t, nil)
	g.store, g.task, g.now = f.store, f.task, f.now
	g.m.store = f.store
	g.m.ctx.Now = func() time.Time { return g.now }
	g.run(g.m.Init())
	if g.m.Phase() != Focus || g.m.Remaining() != 15*time.Minute || g.m.Task().ID != f.task.ID {
		t.Fatalf("restored: %v %v %q", g.m.Phase(), g.m.Remaining(), g.m.Task().Title)
	}

	// If it ran out while closed, it's recorded as completed on launch.
	g.now = g.now.Add(3 * time.Hour)
	h := newFixture(t, nil)
	h.m.store, h.store, h.task = f.store, f.store, f.task
	h.m.ctx.Now = func() time.Time { return g.now }
	h.run(h.m.Init())
	if h.m.Phase() != Idle || !strings.Contains(h.lastStatus(), "finished while tasker was closed") {
		t.Fatalf("finished-while-closed: %v %q", h.m.Phase(), h.lastStatus())
	}
	if got := h.reloadTask(); got.PomodorosDone != 1 || got.FocusTime != 25*time.Minute {
		t.Errorf("credited: done=%d focus=%v", got.PomodorosDone, got.FocusTime)
	}
}

func TestRendering(t *testing.T) {
	f := newFixture(t, nil)
	if f.m.StatusSegment() != "" || f.m.TerminalProgress() != nil {
		t.Error("idle should render nothing")
	}
	f.run(f.m.Start(f.task))
	f.advance(5 * time.Minute)
	if seg := f.m.StatusSegment(); !strings.Contains(seg, "20:00") || !strings.Contains(seg, "Write thesis") {
		t.Errorf("segment = %q", seg)
	}
	if id, label := f.m.RowLabel(); id != f.task.ID || !strings.Contains(label, "20:00") {
		t.Errorf("row label = %q %q", id, label)
	}
	if p := f.m.TerminalProgress(); p == nil || p.Value != 20 {
		t.Errorf("terminal progress = %+v", p)
	}
	view := f.m.FocusView(60, 20)
	for _, want := range []string{"FOCUS", "Write thesis", "space pause", "█"} {
		if !strings.Contains(view, want) {
			t.Errorf("focus view missing %q", want)
		}
	}
}

// Rendering at degenerate sizes (e.g. the first frame, before the terminal
// size is known) must never panic.
func TestRenderingAtZeroSize(t *testing.T) {
	f := newFixture(t, nil)
	f.run(f.m.Start(f.task))
	for _, size := range [][2]int{{0, 0}, {1, 1}, {5, 3}, {-1, -1}} {
		_ = f.m.FocusView(size[0], size[1])
	}
	_ = bar(0.5, -10, f.m.phaseColor(), f.m.phaseColor())
	a := &standalone{m: f.m, task: f.task}
	_ = a.View()
}
