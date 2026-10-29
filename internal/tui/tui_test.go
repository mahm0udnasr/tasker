package tui

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/mahm0udnasr/tasker/internal/domain"
)

func ptr(t time.Time) *time.Time { return &t }

// seedDemo creates a representative data set.
func seedDemo(h *harness) {
	ctx := context.Background()
	p := domain.NewProject("Uni", 0, h.now)
	_ = h.store.CreateProject(ctx, &p)
	ml := h.seed("Finish ML assignment", func(t *domain.Task) {
		t.ProjectID, t.Priority, t.Tags = p.ID, domain.PriorityHigh, []string{"ml", "exam"}
		t.Due, t.DueHasTime = ptr(time.Date(2026, 10, 29, 20, 0, 0, 0, time.Local)), true
		t.Description = "Read the paper first"
	})
	h.seed("Read paper", func(t *domain.Task) { t.ParentID = ml.ID; t.SetStatus(domain.StatusDone, h.now) })
	h.seed("Write code", func(t *domain.Task) { t.ParentID = ml.ID })
	h.seed("Pay rent", func(t *domain.Task) {
		t.Priority = domain.PriorityUrgent
		t.Due = ptr(time.Date(2026, 10, 27, 0, 0, 0, 0, time.Local))
	})
	h.seed("Buy groceries", func(t *domain.Task) { t.Due = ptr(time.Date(2026, 10, 29, 0, 0, 0, 0, time.Local)) })
	h.seed("Plan trip", func(t *domain.Task) { t.Due = ptr(time.Date(2026, 11, 6, 0, 0, 0, 0, time.Local)) })
	h.seed("Call mom", func(t *domain.Task) { t.SetStatus(domain.StatusInProgress, h.now) })
	h.keys("ctrl+r")
}

func TestRendersTodayWithSidebar(t *testing.T) {
	h := newHarness(t, 120, 30)
	seedDemo(h)
	h.dump("today")
	for _, want := range []string{"Today · 3", "Inbox", "Upcoming", "Overdue", "PROJECTS", "Uni",
		"TAGS", "#ml", "Finish ML assignment", "1/2", "Pay rent", "today 20:00", "TODAY", "today"} {
		h.contains(want)
	}
	h.notContains("Plan trip") // upcoming, not today
}

func TestSidebarNavigationChangesScope(t *testing.T) {
	h := newHarness(t, 120, 30)
	seedDemo(h)
	h.keys("h", "j") // focus sidebar, move from Today to Upcoming
	h.contains("Upcoming · 1")
	h.contains("Plan trip")
	h.keys("j") // Overdue
	h.contains("Overdue · 1")
	h.contains("Pay rent")
	h.keys("k", "k", "k") // Inbox
	h.contains("Inbox · 4")
}

func TestAddTaskViaForm(t *testing.T) {
	h := newHarness(t, 120, 36)
	h.contains("Nothing due today")
	h.keys("a")
	h.contains("New task")
	h.dump("form")
	h.contains("→ Thu 29 Oct 2026") // Today scope pre-fills "today"
	h.dump("form")
	h.keys("type:Write report", "ctrl+s")
	h.notContains("New task")
	h.contains("Write report")
	h.contains("Added “Write report”")

	ts, _ := h.store.ListTasks(context.Background(), domain.Filter{View: domain.ViewToday, Now: h.now}, domain.SortDue)
	if len(ts) != 1 || ts[0].Due == nil {
		t.Fatalf("stored: %+v", ts)
	}
}

func TestFormRequiresTitle(t *testing.T) {
	h := newHarness(t, 120, 36)
	h.keys("a", "ctrl+s")
	h.contains("Title is required")
	h.keys("esc")
	h.notContains("New task")
}

func TestToggleDoneAndPriority(t *testing.T) {
	h := newHarness(t, 120, 30)
	seedDemo(h)
	// Cursor starts on the first row (Pay rent, overdue, sorted first).
	h.keys("p") // urgent → none
	h.contains("Priority: none")
	h.keys("x")
	h.contains("Completed “Pay rent”")
	h.contains("Today · 2") // done tasks leave Today
}

func TestDeleteNeedsConfirmation(t *testing.T) {
	h := newHarness(t, 120, 30)
	seedDemo(h)
	h.keys("G", "d") // Finish ML assignment sorts last (due tonight)
	h.contains("Delete task?")
	h.dump("confirm")
	h.keys("enter") // default is "No"
	h.contains("Finish ML assignment")
	h.keys("d", "y")
	h.contains("Deleted “Finish ML assignment”")
	h.notContains("Write code")
}

func TestExpandSubtasksAndAddSubtask(t *testing.T) {
	h := newHarness(t, 120, 30)
	seedDemo(h)
	h.keys("G", "enter")
	h.contains("Write code")
	h.contains("Read paper")
	h.keys("j", "A", "type:Run experiments", "ctrl+s")
	h.contains("Run experiments")
	h.contains("1/3")
	h.dump("subtasks")
}

func TestSetDueAndMove(t *testing.T) {
	h := newHarness(t, 120, 30)
	seedDemo(h)
	h.keys("j", "t") // second row: Buy groceries
	h.contains("Due date · Buy groceries")
	for range 10 {
		h.keys("backspace")
	}
	h.keys("type:next week", "enter")
	h.contains("Due Mon")
	h.notContains("Buy groceries")

	h.keys("g", "m")
	h.contains("Move “Pay rent” to…")
	h.dump("picker")
	h.keys("type:Home", "enter")
	h.contains("Moved to Home · created project Home")
	h.contains("Home")
}

func TestDeleteProjectWithShiftD(t *testing.T) {
	h := newHarness(t, 120, 30)
	seedDemo(h)
	h.keys("D") // Today is not a project
	h.contains("select a project in the sidebar")
	h.keys("d") // lowercase d still deletes the selected task, never a project
	h.contains("Delete task?")
	h.keys("esc")

	h.keys("h", "j", "j", "j", "j") // Today → Upcoming → Overdue → Completed → Uni
	h.contains("Uni · 1")
	h.keys("D")
	h.contains("Delete project?")
	h.contains("1 open task") // the body wraps, so match a fragment
	h.keys("y")
	h.contains("Deleted project “Uni”")
	h.notContains("PROJECTS\n│   Uni")
	h.contains("Inbox · 5") // the ML task moved to the Inbox
	ps, _ := h.store.ListProjects(context.Background())
	if len(ps) != 0 {
		t.Fatalf("projects left: %+v", ps)
	}
}

func TestHelpAndNarrowLayout(t *testing.T) {
	h := newHarness(t, 70, 24)
	seedDemo(h)
	h.notContains("PROJECTS") // sidebar hidden when narrow and list focused
	h.keys("h")
	h.contains("PROJECTS")
	h.keys("l", "?")
	h.contains("Keyboard shortcuts")
	h.dump("help-narrow")
	h.keys("esc")
	h.notContains("Keyboard shortcuts")
}

// Every line must fill the terminal exactly, or borders drift and the
// terminal wraps. Checked across layouts and with overlays open.
func TestScreenGeometry(t *testing.T) {
	for _, size := range [][2]int{{120, 30}, {100, 24}, {79, 20}, {200, 50}} {
		h := newHarness(t, size[0], size[1])
		seedDemo(h)
		check := func(stage string) {
			t.Helper()
			lines := strings.Split(h.screen(), "\n")
			if len(lines) != size[1] {
				t.Errorf("%dx%d %s: %d lines", size[0], size[1], stage, len(lines))
			}
			for i, l := range lines {
				if w := ansi.StringWidth(l); w != size[0] {
					t.Errorf("%dx%d %s: line %d is %d wide: %q", size[0], size[1], stage, i, w, l)
					return
				}
			}
		}
		check("list")
		h.keys("G", "enter")
		check("expanded")
		h.keys("h")
		check("sidebar")
		h.keys("a")
		check("form")
		h.keys("esc", "?")
		check("help")
	}
}

func TestTooSmall(t *testing.T) {
	h := newHarness(t, 30, 8)
	h.contains("Terminal too small")
}
