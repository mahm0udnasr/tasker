package tui

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/mahm0udnasr/tasker/internal/config"
	"github.com/mahm0udnasr/tasker/internal/domain"
	"github.com/mahm0udnasr/tasker/internal/service"
)

// TestScreenshots stages the README screenshots. It only runs when
// TASKER_SNAPSHOT_DIR is set; the .ansi frames are rasterized separately.
func TestScreenshots(t *testing.T) {
	if os.Getenv("TASKER_SNAPSHOT_DIR") == "" {
		t.Skip("set TASKER_SNAPSHOT_DIR to stage screenshots")
	}
	ctx := context.Background()
	cfg := config.Default()
	cfg.Seen = true
	h := newHarnessWithConfig(t, 140, 36, cfg, "")
	if err := service.SeedSamples(ctx, h.store, h.now); err != nil {
		t.Fatal(err)
	}
	// A week of history so the stats charts have something to show.
	for day, n := range []int{2, 4, 1, 3, 5, 2, 3} {
		at := h.now.AddDate(0, 0, day-6)
		for i := range n {
			done := h.seed("Past task", func(t *domain.Task) { t.SetStatus(domain.StatusDone, at.Add(time.Duration(i)*time.Hour)) })
			if i < 2 {
				p := domain.NewPomodoro(done.ID, 25*time.Minute, at.Add(-time.Duration(i+1)*time.Hour))
				_ = h.store.CreatePomodoro(ctx, &p)
				p.Finish(p.Start.Add(25 * time.Minute))
				_ = h.store.UpdatePomodoro(ctx, &p)
			}
		}
	}
	h.keys("ctrl+r", "j", "j", "j", "enter", "j")
	h.dump("shot-main")

	h.keys("k", "k", "P")
	h.now = h.now.Add(8*time.Minute + 23*time.Second)
	h.keys("z")
	h.dump("shot-focus")
	h.keys("esc", "X", "S")
	h.dump("shot-stats")
	h.keys("esc", "T", "down", "down")
	h.dump("shot-themes")

	// Habits with a varied history: a long streak, a gap, a fresh habit.
	habits := []struct {
		name string
		done []int // days ago
	}{
		{"Read 20 pages", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11}},
		{"Morning stretch", []int{1, 2, 3, 5, 6}},
		{"Drink 2L of water", []int{0, 1, 3, 4, 5, 6}},
		{"No phone after 10pm", []int{2, 3, 6}},
		{"Practice Spanish", []int{0}},
	}
	for i, hb := range habits {
		x := domain.NewHabit(hb.name, h.now.Add(time.Duration(i)*time.Second))
		if err := h.store.CreateHabit(ctx, &x); err != nil {
			t.Fatal(err)
		}
		for _, d := range hb.done {
			_ = h.store.SetHabitDone(ctx, x.ID, domain.DayKey(h.now.AddDate(0, 0, -d)), true, h.now)
		}
	}
	h.keys("esc", "ctrl+p", "type:go to habits", "enter", "ctrl+r", "j")
	h.dump("shot-habits")
	h.keys("?")
	h.dump("shot-help")
}
