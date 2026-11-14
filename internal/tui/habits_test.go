package tui

import (
	"context"
	"testing"

	"github.com/mahm0udnasr/tasker/internal/domain"
	"github.com/mahm0udnasr/tasker/internal/theme"
)

// flame prefixes the streak (the default config uses Nerd Font icons).
var flame = theme.NerdIcons.Flame

// openHabits moves from the starting Today view to Habits and focuses the list.
func openHabits(h *harness) {
	h.keys("h", "j", "j", "j", "j", "l") // Today → … → Completed → Habits
	h.contains("HABITS")
}

func TestHabitsEmptyState(t *testing.T) {
	h := newHarness(t, 120, 30)
	seedDemo(h)
	openHabits(h)
	h.contains("Habits · 0")
	h.contains("No habits yet")
	h.contains("press a to add a habit")
	h.notContains("Finish ML assignment")
}

func TestHabitLifecycle(t *testing.T) {
	h := newHarness(t, 160, 30) // wide enough for the status bar hints
	openHabits(h)
	h.contains("a add habit") // status bar hints
	h.contains("x/space toggle today")

	h.keys("a", "type:Read", "enter")
	h.contains("Added habit “Read”")
	h.contains("Habits · 1 · 0/1 today")

	// Done yesterday: the streak holds while today is still open.
	ctx := context.Background()
	hs, _ := h.store.ListHabits(ctx)
	if len(hs) != 1 {
		t.Fatalf("habits = %+v", hs)
	}
	yesterday := domain.DayKey(h.now.AddDate(0, 0, -1))
	if err := h.store.SetHabitDone(ctx, hs[0].ID, yesterday, true, h.now); err != nil {
		t.Fatal(err)
	}
	h.keys("ctrl+r")
	h.contains(flame + " 1")

	h.keys("x")
	h.contains("“Read” done today · 2-day streak")
	h.contains("Habits · 1 · 1/1 today")
	h.contains(flame + " 2")

	h.keys("space") // toggling again undoes today
	h.contains("“Read” not done today")
	h.contains("0/1 today")
	h.keys("u") // and undo restores it
	h.contains("Undid:")
	h.contains("1/1 today")

	h.keys("e", "ctrl+u", "type:Read 20 pages", "enter")
	h.contains("Renamed “Read” to “Read 20 pages”")

	h.keys("p") // task-only keys are refused, not applied to a hidden task
	h.contains("isn't available in Habits")

	h.keys("d")
	h.contains("Delete habit?")
	h.keys("y")
	h.contains("Deleted habit “Read 20 pages”")
	h.contains("No habits yet")

	h.keys("u")
	h.contains("Read 20 pages")
	h.contains(flame + " 2") // history came back with it
}

func TestHabitNamesAreUnique(t *testing.T) {
	h := newHarness(t, 120, 30)
	openHabits(h)
	h.keys("a", "type:Stretch", "enter")
	h.keys("a", "type:stretch", "enter")
	h.contains("name already exists")
}

func TestHabitsInHelpAndPalette(t *testing.T) {
	h := newHarness(t, 120, 40)
	h.keys("?")
	h.contains("Habits")
	h.contains("toggle today")
	h.keys("esc", "ctrl+p", "type:go to habits", "enter")
	h.contains("HABITS")
	h.contains("No habits yet")
}
