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

// formWithNames opens the add form on Project, with projects and tags to offer.
func formWithNames(t *testing.T) *harness {
	t.Helper()
	h := newHarness(t, 120, 40)
	for _, name := range []string{"University", "Uni", "Home"} {
		p := domain.NewProject(name, 0, h.now)
		if err := h.store.CreateProject(context.Background(), &p); err != nil {
			t.Fatal(err)
		}
	}
	h.seed("Old", func(tk *domain.Task) { tk.Tags = []string{"work", "personal", "workshop"} })
	h.keys("ctrl+r", "a", "type:New", "tab", "tab", "tab")
	return h
}

func savedTask(t *testing.T, h *harness) domain.Task {
	t.Helper()
	ts, err := h.store.ListTasks(context.Background(), domain.Filter{IncludeDone: true}, domain.SortManual)
	if err != nil {
		t.Fatal(err)
	}
	for _, task := range ts {
		if task.Title == "New" {
			return task
		}
	}
	t.Fatalf("task not saved: %+v", ts)
	return domain.Task{}
}

func TestFormCompletesProject(t *testing.T) {
	h := formWithNames(t)
	h.keys("type:ho")
	h.contains("> home")
	h.contains("tab complete")
	h.keys("tab", "ctrl+s")
	h.contains("Added “New”")
	h.notContains("created project")
	task := savedTask(t, h)
	if p, err := h.store.GetProject(context.Background(), task.ProjectID); err != nil || p.Name != "Home" {
		t.Fatalf("project: %+v, %v", p, err)
	}
}

func TestFormTabLeavesExactProject(t *testing.T) {
	h := formWithNames(t)
	h.keys("type:Uni", "tab", "type:x", "ctrl+s") // "x" lands in Tags, not "University"
	h.notContains("created project")
	if task := savedTask(t, h); len(task.Tags) != 1 || task.Tags[0] != "x" {
		t.Fatalf("tags: %+v", task.Tags)
	}
}

func TestFormNewProjectStaysAsTyped(t *testing.T) {
	h := formWithNames(t)
	h.keys("type:Zed")
	h.notContains("tab complete")
	h.keys("tab", "ctrl+s")
	h.contains("created project Zed")
}

func TestFormCompletesTags(t *testing.T) {
	h := formWithNames(t)
	h.keys("tab", "type:pe")
	h.contains("> personal")
	h.keys("tab")
	h.contains("> personal")
	h.keys("type:, wo")
	h.contains("> personal, work")
	h.contains("↑/↓ other matches")
	h.keys("down")
	h.contains("> personal, workshop")
	h.keys("tab", "ctrl+s")
	task := savedTask(t, h)
	if strings.Join(task.Tags, ",") != "personal,workshop" {
		t.Fatalf("tags: %+v", task.Tags)
	}
}

func TestFormDoesNotOfferUsedOrStaleTags(t *testing.T) {
	h := formWithNames(t)
	h.keys("tab", "type:personal, p")
	h.notContains("tab complete") // already used
	h.keys("backspace")
	h.notContains("tab complete")
	h.keys("type:w", "backspace", "backspace", "backspace")
	h.notContains("tab complete")
	h.keys("tab") // nothing to complete: moves on to Estimate
	h.keys("type:3", "ctrl+s")
	if task := savedTask(t, h); task.Estimate != 3 {
		t.Fatalf("estimate: %d", task.Estimate)
	}
}

func TestFormHasNoNotesField(t *testing.T) {
	h := newHarness(t, 120, 40)
	h.keys("a")
	h.contains("New task")
	h.notContains("Notes")
}

func TestNotesFromDetails(t *testing.T) {
	h := newHarness(t, 120, 40)
	h.keys("a", "type:Write report", "ctrl+s")
	h.contains("No notes. Press n to add some.")

	h.keys("n")
	h.contains("Notes · Write report")
	h.keys("type:line one", "enter", "type:line two", "enter", "enter",
		"type:- item a", "enter", "type:- item b", "ctrl+s")
	h.contains("Saved notes of “Write report”")
	h.contains("line two")
	h.contains("• item b")
	h.notContains("line one line two")

	ctx := context.Background()
	ts, _ := h.store.ListTasks(ctx, domain.Filter{IncludeDone: true}, domain.SortManual)
	if len(ts) != 1 || ts[0].Description != "line one\nline two\n\n- item a\n- item b" {
		t.Fatalf("notes not saved as typed: %+v", ts)
	}

	// The details pane can open the editor too, and it starts from the notes.
	h.keys("l", "n", "enter", "type:- item c", "ctrl+s")
	h.contains("• item c")

	// Undo restores the previous notes; editing the task keeps them.
	h.keys("u")
	h.notContains("item c")
	h.keys("e", "type: v2", "ctrl+s")
	got, err := h.store.GetTask(ctx, ts[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "Write report v2" || got.Description != ts[0].Description {
		t.Fatalf("edit form changed notes: %+v", got)
	}
}

func TestNotesCancelAndClear(t *testing.T) {
	h := newHarness(t, 120, 40)
	h.seed("Noted", func(tk *domain.Task) { tk.Description = "keep me"; tk.SetDue(&h.now, false, h.now) })
	h.keys("ctrl+r", "n")
	h.contains("Notes · Noted")
	h.keys("type:!!!", "esc")
	h.notContains("!!!")
	h.contains("keep me")

	h.keys("n", "ctrl+s") // no change: nothing to save or undo
	h.notContains("Saved notes")

	h.keys("n")
	for range len("keep me") {
		h.keys("backspace")
	}
	h.keys("ctrl+s")
	h.contains("Cleared notes of “Noted”")
	h.contains("No notes.")
}

func TestNotesKeepLongText(t *testing.T) {
	h := newHarness(t, 120, 40)
	long := strings.TrimSpace(strings.Repeat("line\n", 150))
	task := h.seed("Long", func(tk *domain.Task) { tk.Description = long; tk.SetDue(&h.now, false, h.now) })
	h.keys("ctrl+r", "n", "enter", "type:more", "ctrl+s")
	got, err := h.store.GetTask(context.Background(), task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Description != long+"\nmore" {
		t.Fatalf("notes cut to %d lines", strings.Count(got.Description, "\n")+1)
	}
}

func TestNotesFromNarrowDetailModal(t *testing.T) {
	h := newHarness(t, 70, 24)
	h.seed("Noted", func(tk *domain.Task) { tk.SetDue(&h.now, false, h.now) })
	h.keys("ctrl+r", "v")
	h.contains("esc close")
	h.keys("n")
	h.notContains("esc close")
	h.contains("Notes · Noted")
	h.keys("type:hello", "ctrl+s")
	h.contains("Saved notes of “Noted”")
}

func TestToggleDoneAndPriority(t *testing.T) {
	h := newHarness(t, 120, 30)
	seedDemo(h)
	// Cursor starts on the first row (Pay rent, overdue, sorted first).
	h.keys("p") // urgent → none
	h.contains("Priority of “Pay rent”: none")
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
	h.contains("“Buy groceries” due Mon")
	h.contains("Today · 2") // it moved to next week

	h.keys("g", "m")
	h.contains("Move “Pay rent” to…")
	h.dump("picker")
	h.keys("type:Home", "enter")
	h.contains("Moved “Pay rent” to Home · created project Home")
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

func TestTinyAndZeroSizesNeverPanic(t *testing.T) {
	for _, size := range [][2]int{{0, 0}, {1, 1}, {10, 3}, {39, 9}, {40, 10}} {
		h := newHarness(t, size[0], size[1])
		seedDemo(h)
		h.keys("G", "v", "esc", "P", "z", "esc", "/", "esc", "?", "esc")
		_ = h.screen()
	}
}
