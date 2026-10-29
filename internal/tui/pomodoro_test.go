package tui

import (
	"context"
	"testing"
	"time"

	"github.com/mahm0udnasr/tasker/internal/domain"
)

func TestPomodoroInTUI(t *testing.T) {
	h := newHarness(t, 140, 32)
	seedDemo(h)
	h.keys("G") // Finish ML assignment
	h.keys("P")
	h.contains("Focus on “Finish ML assignment” — 25:00")
	h.contains("25:00 Finish ML") // status-bar timer

	h.now = h.now.Add(7 * time.Minute) // time passes; the next render shows it
	h.contains("18:00")

	h.keys("z")
	h.contains("FOCUS")
	h.contains("space pause")
	h.dump("focus-mode")
	h.keys("space")
	h.contains("FOCUS · PAUSED")
	h.keys("esc")
	h.notContains("space pause")
	h.dump("pomodoro-running")

	h.keys("P") // resume from the list
	h.keys("X")
	h.contains("Stopped · 7m of focus on “Finish ML assignment”")
	h.contains("7m") // detail pane: focus time
	tasks, _ := h.store.ListTasks(context.Background(), domain.Filter{View: domain.ViewToday, Now: h.now}, domain.SortDue)
	for _, x := range tasks {
		if x.Title == "Finish ML assignment" && x.FocusTime != 7*time.Minute {
			t.Errorf("focus time = %v", x.FocusTime)
		}
	}
}

func TestPomodoroRestoredOnLaunch(t *testing.T) {
	ctx := context.Background()
	h := newHarness(t, 140, 32)
	task := h.seed("Long task", func(t *domain.Task) { t.Estimate = 4 })
	p := domain.NewPomodoro(task.ID, 25*time.Minute, h.now.Add(-5*time.Minute))
	if err := h.store.CreatePomodoro(ctx, &p); err != nil {
		t.Fatal(err)
	}

	// A fresh model on the same database picks up the running session.
	h2 := &harness{t: t, store: h.store, now: h.now}
	h2.m = New(h.store, configDefault(), func() time.Time { return h2.now })
	h2.send(teaSize(140, 32))
	h2.run(h2.m.Init())
	h2.keys("h", "k", "l") // Inbox, where the task lives
	h2.contains("20:00 Long task")
	h2.contains("0/4") // estimate shown on the row
}

func TestEstimateInForm(t *testing.T) {
	h := newHarness(t, 120, 40)
	h.keys("a", "type:Big project")
	for range 5 { // title → notes → due → priority → project → tags → estimate
		h.keys("tab")
	}
	h.keys("tab", "type:6", "ctrl+s")
	h.contains("Added “Big project”")
	ts, _ := h.store.ListTasks(context.Background(), domain.Filter{IncludeDone: true}, domain.SortManual)
	if len(ts) != 1 || ts[0].Estimate != 6 {
		t.Fatalf("estimate not saved: %+v", ts)
	}
	h.keys("e")
	h.contains("6") // pre-filled when editing
	h.keys("esc")
}

func TestPomodoroPaletteAndHelp(t *testing.T) {
	h := newHarness(t, 140, 40)
	seedDemo(h)
	h.keys("ctrl+p", "type:start / pause pomo", "enter")
	h.contains("Focus on “Pay rent”")
	h.keys("?")
	h.contains("Pomodoro")
	h.contains("focus mode")
	h.keys("esc")
}

func TestPomodoroGeometry(t *testing.T) {
	for _, size := range [][2]int{{140, 32}, {100, 24}, {70, 20}, {45, 12}} {
		h := newHarness(t, size[0], size[1])
		seedDemo(h)
		h.keys("G", "P")
		assertGeometry(t, h, size[0], size[1])
		h.keys("z")
		assertGeometry(t, h, size[0], size[1])
		h.keys("space")
		assertGeometry(t, h, size[0], size[1])
	}
}
