package tui

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/mahm0udnasr/tasker/internal/config"
	"github.com/mahm0udnasr/tasker/internal/domain"
)

// newHarnessWithConfig is newHarness with a real config file, so saving
// preferences and the first-run welcome can be tested.
func newHarnessWithConfig(t *testing.T, w, ht int, cfg config.Config, path string) *harness {
	t.Helper()
	h := newHarness(t, w, ht) // for the store
	h2 := &harness{t: t, store: h.store, now: h.now}
	h2.m = NewWithConfigPath(h.store, cfg, path, func() time.Time { return h2.now })
	h2.send(tea.WindowSizeMsg{Width: w, Height: ht})
	h2.run(h2.m.Init())
	return h2
}

func TestThemePickerPreviewsRevertsAndSaves(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	_ = os.WriteFile(path, []byte("# my settings\ntheme = \"catppuccin-mocha\"\nseen_welcome = true\n"), 0o600)
	cfg, _, _ := config.Load(path)
	h := newHarnessWithConfig(t, 120, 30, cfg, path)
	seedDemo(h)
	theme := func() string { return h.m.(Model).ctx.Styles.Theme.Name }

	h.keys("T")
	h.contains("Theme")
	h.contains("catppuccin-latte")
	h.dump("theme-picker")
	h.keys("down") // preview the next theme live
	if theme() == "catppuccin-mocha" {
		t.Fatal("moving in the picker should preview")
	}
	h.keys("esc") // cancel → back to the original
	if theme() != "catppuccin-mocha" {
		t.Fatalf("cancel should revert, theme = %s", theme())
	}

	h.keys("T", "type:nord", "enter")
	if theme() != "nord" {
		t.Fatalf("picked theme = %s", theme())
	}
	h.contains("Theme: nord")
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "theme = \"nord\"") || !strings.Contains(string(data), "# my settings") {
		t.Errorf("config not saved in place:\n%s", data)
	}
}

func TestEveryThemeRenders(t *testing.T) {
	for _, name := range []string{"catppuccin-latte", "tokyo-night", "gruvbox", "nord"} {
		cfg := config.Default()
		cfg.Theme, cfg.Seen = name, true
		h := newHarnessWithConfig(t, 140, 32, cfg, "")
		seedDemo(h)
		h.keys("G")
		h.contains("Finish ML assignment")
		assertGeometry(t, h, 140, 32)
		h.dump("theme-" + name)
	}
}

func TestStatsView(t *testing.T) {
	ctx := context.Background()
	h := newHarness(t, 120, 40)
	seedDemo(h)
	// Completions today and yesterday → a 2-day streak.
	for i, offset := range []int{0, 0, -1} {
		at := h.now.AddDate(0, 0, offset)
		h.seed("done "+string(rune('a'+i)), func(t *domain.Task) { t.SetStatus(domain.StatusDone, at) })
	}
	task := h.seed("focused", nil)
	p := domain.NewPomodoro(task.ID, 25*time.Minute, h.now.Add(-time.Hour))
	_ = h.store.CreatePomodoro(ctx, &p)
	p.Finish(h.now)
	_ = h.store.UpdatePomodoro(ctx, &p)

	h.keys("S")
	h.dump("stats")
	for _, want := range []string{"Your week", "2 days", "streak", "3", "this week",
		"25m", "focus time", "Tasks completed", "Focus time", "today", "1 pomodoro this week", "best streak 2 days"} {
		h.contains(want)
	}
	assertGeometry(t, h, 120, 40)
	h.keys("esc")
	h.notContains("Your week")
}

func TestWelcomeOnFirstRun(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cfg", "config.toml")
	h := newHarnessWithConfig(t, 120, 36, config.Default(), path)
	h.contains("Welcome!")
	h.dump("welcome")
	h.keys("s") // add the examples
	h.notContains("Welcome!")
	h.contains("Added example tasks")
	h.contains("Getting started")
	h.contains("Complete me")

	// The commented config was written and marks the welcome as seen.
	cfg, exists, err := config.Load(path)
	if err != nil || !exists || !cfg.Seen {
		t.Fatalf("config after welcome: %+v exists=%v err=%v", cfg, exists, err)
	}
	h2 := newHarnessWithConfig(t, 120, 36, cfg, path)
	h2.notContains("Welcome!")
}

func TestWelcomeAnyKeyStartsFresh(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	h := newHarnessWithConfig(t, 120, 36, config.Default(), path)
	h.keys("enter")
	h.notContains("Welcome!")
	ts, _ := h.store.ListTasks(context.Background(), domain.Filter{IncludeDone: true}, domain.SortManual)
	if len(ts) != 0 {
		t.Errorf("no samples expected, got %d tasks", len(ts))
	}
}
