package tui

import (
	"context"
	"testing"
	"time"

	"github.com/mahm0udnasr/tasker/internal/domain"
)

func TestDetailPaneFollowsSelection(t *testing.T) {
	h := newHarness(t, 140, 32)
	seedDemo(h)
	h.contains("Details")
	h.contains("Pay rent") // first row selected
	h.contains("2 days ago")
	h.keys("G") // Finish ML assignment: has notes and subtasks
	h.contains("Subtasks 1/2")
	h.contains("Read the paper first") // description rendered
	h.contains("Uni")
	h.dump("detail")

	h.keys("v") // hide
	h.notContains("Details")
	h.keys("v") // show again
	h.contains("Subtasks 1/2")
}

func TestDetailOpensAsModalWhenNarrow(t *testing.T) {
	h := newHarness(t, 100, 30)
	seedDemo(h)
	h.notContains("Subtasks")
	h.keys("G", "v")
	h.contains("Subtasks 1/2")
	h.contains("esc close")
	h.keys("esc")
	h.notContains("Subtasks 1/2")
}

func TestSearchJumpsToTask(t *testing.T) {
	h := newHarness(t, 120, 30)
	seedDemo(h)
	h.keys("/")
	h.contains("Search 7 tasks")
	h.keys("type:plan", "enter") // Plan trip lives in the Inbox (upcoming)
	h.contains("Inbox · 4")

	h.keys("/", "type:write code")
	h.dump("search")
	h.keys("enter") // subtask: jumps to its parent's project, expanded
	h.contains("Uni · 1")
	h.contains("Write code")
	h.contains("Read paper")

	h.keys("/", "type:#exam") // tags are searchable too
	h.contains("Finish ML assignment")
	h.keys("esc")
}

func TestFilterAndClear(t *testing.T) {
	h := newHarness(t, 120, 30)
	seedDemo(h)
	h.keys("f")
	h.contains("Filter this list")
	h.dump("filter")
	h.keys("right", "right", "right", "enter") // priority: high+
	h.contains("Filtered: ≥high")
	h.contains("Today · 2 · ⧩ ≥high")
	h.contains("Pay rent")
	h.keys("F")
	h.contains("Filter cleared")
	h.contains("Today · 3")
}

func TestSortCycleAndReorder(t *testing.T) {
	h := newHarness(t, 120, 30)
	seedDemo(h)
	h.keys("h", "k", "l") // Inbox, manual order
	h.contains("Inbox · 4")
	h.keys("J") // move first task (Pay rent) down
	order := func() []string {
		ts, _ := h.store.ListTasks(context.Background(), domain.Filter{View: domain.ViewInbox, Now: h.now}, domain.SortManual)
		var out []string
		for _, x := range ts {
			out = append(out, x.Title)
		}
		return out
	}
	if got := order(); got[0] != "Buy groceries" || got[1] != "Pay rent" {
		t.Fatalf("after J: %v", got)
	}
	h.keys("u") // reorder is undoable
	if got := order(); got[0] != "Pay rent" {
		t.Fatalf("after undo: %v", got)
	}

	h.keys("s")
	h.contains("Sorted by due")
	h.contains("Inbox · 4 · ↕ due")
	h.keys("J")
	h.contains("reordering needs manual sort")
}

func TestPalette(t *testing.T) {
	h := newHarness(t, 120, 30)
	seedDemo(h)
	h.keys("ctrl+p")
	h.contains("Command palette")
	h.dump("palette")
	h.keys("type:go to upco", "enter")
	h.contains("Upcoming · 1")
	h.keys("ctrl+p", "type:toggle done", "enter")
	h.contains("Completed “Plan trip”")
	h.keys(":", "type:go to project uni", "enter") // ":" opens the palette too
	h.contains("Uni · 1")
}

func TestUndo(t *testing.T) {
	ctx := context.Background()
	h := newHarness(t, 120, 30)
	seedDemo(h)
	h.keys("u")
	h.contains("Nothing to undo")

	// Delete with subtasks, then undo.
	h.keys("G", "d", "y")
	h.contains("Deleted “Finish ML assignment” · u to undo")
	h.keys("u")
	h.contains("Undid: Deleted “Finish ML assignment”")
	h.contains("Finish ML assignment")
	got, err := h.store.ListTasks(ctx, domain.Filter{IncludeSubtasks: true, IncludeDone: true}, domain.SortManual)
	if err != nil || len(got) != 7 {
		t.Fatalf("after undo delete: %d tasks, %v", len(got), err)
	}

	// Toggle done, then undo.
	h.keys("g", "x")
	h.contains("Today · 2")
	h.keys("u")
	h.contains("Today · 3")

	// Add, then undo removes it.
	h.keys("a", "type:Temporary", "ctrl+s")
	h.contains("Today · 4")
	h.keys("u")
	h.contains("Today · 3")

	// Move into a new project, then undo removes the project too.
	h.keys("g", "m", "type:Errands", "enter")
	h.contains("created project Errands")
	h.keys("u")
	ps, _ := h.store.ListProjects(ctx)
	if len(ps) != 1 {
		t.Fatalf("project created by the move should be gone: %+v", ps)
	}

	// Delete a project, then undo restores it and its tasks.
	h.keys("h", "j", "j", "j", "j", "D", "y")
	h.contains("Deleted project “Uni”")
	h.keys("u")
	ps, _ = h.store.ListProjects(ctx)
	ml, _ := h.store.ListTasks(ctx, domain.Filter{ProjectID: ps[0].ID, IncludeSubtasks: true, IncludeDone: true}, domain.SortManual)
	if len(ps) != 1 || len(ml) != 3 {
		t.Fatalf("after undo project delete: %d projects, %d tasks", len(ps), len(ml))
	}
}

func TestPhase3Geometry(t *testing.T) {
	for _, size := range [][2]int{{140, 32}, {120, 30}, {100, 24}, {70, 20}} {
		h := newHarness(t, size[0], size[1])
		seedDemo(h)
		for _, keys := range [][]string{{"G"}, {"v"}, {"esc", "/"}, {"esc", "f"}, {"esc", "ctrl+p"}, {"esc"}} {
			h.keys(keys...)
			assertGeometry(t, h, size[0], size[1])
		}
	}
}

func TestDetailRendersMarkdown(t *testing.T) {
	h := newHarness(t, 140, 40)
	h.seed("Write thesis chapter", func(t *domain.Task) {
		t.Due = ptr(h.now.Add(26 * time.Hour))
		t.DueHasTime = true
		t.Description = "## Plan\n\n1. Outline\n2. **Draft** the _intro_\n\n- [x] collect sources\n- [ ] ask supervisor\n\n" +
			"> Keep it under 20 pages.\n\nRun `make figures` first. See [the guide](https://example.com)."
	})
	h.keys("h", "k", "l") // Inbox
	h.contains("Plan")
	h.contains("collect sources")
	h.contains("make figures")
	h.notContains("**Draft**") // markdown syntax is rendered, not shown raw
	h.dump("markdown")
}
