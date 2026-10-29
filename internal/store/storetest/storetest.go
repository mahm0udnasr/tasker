// Package storetest is a conformance suite for store.Store implementations.
// Backends call Run from their own tests with a factory that returns a fresh,
// empty store.
package storetest

import (
	"context"
	"errors"
	"sort"
	"testing"
	"time"

	"github.com/mahm0udnasr/tasker/internal/domain"
	"github.com/mahm0udnasr/tasker/internal/store"
)

// Factory returns an empty store; the suite closes it.
type Factory func(t *testing.T) store.Store

// Thursday 29 Oct 2026, 14:30 local.
var now = time.Date(2026, 10, 29, 14, 30, 0, 0, time.Local)

func ptr(t time.Time) *time.Time { return &t }

// Run executes every conformance test.
func Run(t *testing.T, newStore Factory) {
	tests := []struct {
		name string
		fn   func(*testing.T, store.Store)
	}{
		{"CreateAndGet", testCreateAndGet},
		{"GetByPrefix", testGetByPrefix},
		{"UpdateTask", testUpdateTask},
		{"Validation", testValidation},
		{"Subtasks", testSubtasks},
		{"SoftDeleteAndRestore", testSoftDeleteAndRestore},
		{"Projects", testProjects},
		{"FiltersAgreeWithDomain", testFiltersAgreeWithDomain},
		{"Sorting", testSorting},
		{"CountsAndTags", testCountsAndTags},
		{"Stats", testStats},
		{"ChangesSince", testChangesSince},
		{"Pomodoros", testPomodoros},
		{"Estimate", testEstimate},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newStore(t)
			t.Cleanup(func() { _ = s.Close() })
			tt.fn(t, s)
		})
	}
}

func mustCreate(t *testing.T, s store.Store, title string, mod func(*domain.Task)) domain.Task {
	t.Helper()
	task := domain.NewTask(title, now)
	if mod != nil {
		mod(&task)
	}
	if err := s.CreateTask(context.Background(), &task); err != nil {
		t.Fatalf("CreateTask(%q): %v", title, err)
	}
	return task
}

func mustProject(t *testing.T, s store.Store, name string) domain.Project {
	t.Helper()
	p := domain.NewProject(name, 0, now)
	if err := s.CreateProject(context.Background(), &p); err != nil {
		t.Fatalf("CreateProject(%q): %v", name, err)
	}
	return p
}

func ids(ts []domain.Task) []string {
	out := make([]string, len(ts))
	for i, t := range ts {
		out[i] = t.ID
	}
	return out
}

func testCreateAndGet(t *testing.T, s store.Store) {
	ctx := context.Background()
	p := mustProject(t, s, "Uni")
	created := mustCreate(t, s, "  Finish ML assignment ", func(task *domain.Task) {
		task.Description = "## Notes\n- read paper"
		task.Priority = domain.PriorityHigh
		task.Due = ptr(time.Date(2026, 10, 30, 20, 0, 0, 0, time.Local))
		task.DueHasTime = true
		task.ProjectID = p.ID
		task.Tags = []string{"ML", "#exam", "ml"}
	})

	got, err := s.GetTask(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "Finish ML assignment" {
		t.Errorf("title not trimmed: %q", got.Title)
	}
	if got.Priority != domain.PriorityHigh || got.ProjectID != p.ID || got.Status != domain.StatusTodo {
		t.Errorf("fields not round-tripped: %+v", got)
	}
	if got.Due == nil || !got.Due.Equal(*created.Due) || !got.DueHasTime {
		t.Errorf("due = %v hasTime=%v", got.Due, got.DueHasTime)
	}
	if len(got.Tags) != 2 || got.Tags[0] != "exam" || got.Tags[1] != "ml" {
		t.Errorf("tags = %v", got.Tags)
	}
	if !got.CreatedAt.Equal(now) || got.SortOrder == 0 {
		t.Errorf("created=%v sort=%v", got.CreatedAt, got.SortOrder)
	}

	second := mustCreate(t, s, "second", nil)
	if second.SortOrder <= got.SortOrder {
		t.Errorf("new tasks should append: %v <= %v", second.SortOrder, got.SortOrder)
	}

	if _, err := s.GetTask(ctx, "ffffffff-0000-0000-0000-000000000000"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("missing task err = %v", err)
	}
}

func testGetByPrefix(t *testing.T, s store.Store) {
	ctx := context.Background()
	a := mustCreate(t, s, "a", func(t *domain.Task) { t.ID = "abcd1111-0000-4000-8000-000000000000" })
	mustCreate(t, s, "b", func(t *domain.Task) { t.ID = "abcd2222-0000-4000-8000-000000000000" })

	tests := []struct {
		prefix string
		want   string
		err    error
	}{
		{"abcd1", a.ID, nil},
		{"ABCD1111", a.ID, nil},
		{"abcd", "", store.ErrAmbiguousID},
		{"abc", "", store.ErrNotFound},    // too short
		{"zzzzzz", "", store.ErrNotFound}, // not hex
		{"abcd%", "", store.ErrNotFound},  // no LIKE injection
	}
	for _, tt := range tests {
		got, err := s.GetTask(ctx, tt.prefix)
		if !errors.Is(err, tt.err) || got.ID != tt.want {
			t.Errorf("GetTask(%q) = %q, %v; want %q, %v", tt.prefix, got.ID, err, tt.want, tt.err)
		}
	}
}

func testUpdateTask(t *testing.T, s store.Store) {
	ctx := context.Background()
	task := mustCreate(t, s, "draft", func(t *domain.Task) { t.Tags = []string{"a", "b"} })

	later := now.Add(time.Hour)
	task.Title = "final"
	task.SetStatus(domain.StatusDone, later)
	task.SetDue(nil, false, later)
	task.Tags = []string{"c"}
	if err := s.UpdateTask(ctx, &task); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetTask(ctx, task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "final" || got.Status != domain.StatusDone || got.CompletedAt == nil ||
		!got.UpdatedAt.Equal(later) || len(got.Tags) != 1 || got.Tags[0] != "c" {
		t.Errorf("update not persisted: %+v", got)
	}

	ghost := domain.NewTask("ghost", now)
	if err := s.UpdateTask(ctx, &ghost); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("update missing = %v", err)
	}
}

func testValidation(t *testing.T, s store.Store) {
	ctx := context.Background()
	tests := []struct {
		name string
		mod  func(*domain.Task)
		want error
	}{
		{"empty title", func(t *domain.Task) { t.Title = "" }, domain.ErrEmptyTitle},
		{"bad priority", func(t *domain.Task) { t.Priority = 7 }, domain.ErrInvalidPrio},
		{"missing parent", func(t *domain.Task) { t.ParentID = domain.NewID() }, store.ErrInvalidParent},
		{"missing project", func(t *domain.Task) { t.ProjectID = domain.NewID() }, store.ErrNotFound},
	}
	for _, tt := range tests {
		task := domain.NewTask("x", now)
		tt.mod(&task)
		if err := s.CreateTask(ctx, &task); !errors.Is(err, tt.want) {
			t.Errorf("%s: err = %v, want %v", tt.name, err, tt.want)
		}
	}
}

func testSubtasks(t *testing.T, s store.Store) {
	ctx := context.Background()
	p := mustProject(t, s, "Home")
	parent := mustCreate(t, s, "parent", func(t *domain.Task) { t.ProjectID = p.ID })
	sub1 := mustCreate(t, s, "sub1", func(t *domain.Task) { t.ParentID = parent.ID })
	mustCreate(t, s, "sub2", func(t *domain.Task) { t.ParentID = parent.ID; t.SetStatus(domain.StatusDone, now) })

	if sub1.ProjectID != p.ID {
		t.Errorf("subtask should inherit project, got %q", sub1.ProjectID)
	}

	got, _ := s.GetTask(ctx, parent.ID)
	if got.SubtaskProgress() != "1/2" {
		t.Errorf("progress = %q", got.SubtaskProgress())
	}

	top, _ := s.ListTasks(ctx, domain.Filter{IncludeDone: true}, domain.SortManual)
	if len(top) != 1 {
		t.Errorf("top-level list should hide subtasks, got %d", len(top))
	}
	subs, _ := s.ListTasks(ctx, domain.Filter{ParentID: parent.ID, IncludeDone: true}, domain.SortManual)
	if len(subs) != 2 {
		t.Errorf("subtasks = %d", len(subs))
	}

	// One level only.
	nested := domain.NewTask("nested", now)
	nested.ParentID = sub1.ID
	if err := s.CreateTask(ctx, &nested); !errors.Is(err, domain.ErrNestedSubtask) {
		t.Errorf("nested subtask err = %v", err)
	}
	// A parent cannot become a subtask.
	other := mustCreate(t, s, "other", nil)
	parent.ParentID = other.ID
	if err := s.UpdateTask(ctx, &parent); !errors.Is(err, store.ErrInvalidParent) {
		t.Errorf("parent→subtask err = %v", err)
	}

	// Moving the parent moves its subtasks.
	parent, _ = s.GetTask(ctx, parent.ID)
	parent.ProjectID = ""
	if err := s.UpdateTask(ctx, &parent); err != nil {
		t.Fatal(err)
	}
	sub1, _ = s.GetTask(ctx, sub1.ID)
	if sub1.ProjectID != "" {
		t.Errorf("subtask project not updated: %q", sub1.ProjectID)
	}
}

func testSoftDeleteAndRestore(t *testing.T, s store.Store) {
	ctx := context.Background()
	parent := mustCreate(t, s, "parent", func(t *domain.Task) { t.Tags = []string{"keep"} })
	sub := mustCreate(t, s, "sub", func(t *domain.Task) { t.ParentID = parent.ID })
	snapshot := []domain.Task{parent, sub}

	if err := s.DeleteTask(ctx, parent.ID, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetTask(ctx, sub.ID); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("subtask should be deleted with parent: %v", err)
	}
	if err := s.DeleteTask(ctx, parent.ID, now); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("double delete err = %v", err)
	}

	// Tombstones are visible to sync.
	cs, err := s.ChangesSince(ctx, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(cs.Tasks) != 2 || cs.Tasks[0].DeletedAt == nil {
		t.Errorf("changes = %+v", cs.Tasks)
	}

	// Undo: restore the pre-delete snapshot.
	if err := s.RestoreTasks(ctx, snapshot); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetTask(ctx, parent.ID)
	if err != nil {
		t.Fatalf("restored parent: %v", err)
	}
	if got.SubtasksTotal != 1 || len(got.Tags) != 1 {
		t.Errorf("restore incomplete: %+v", got)
	}
}

func testProjects(t *testing.T, s store.Store) {
	ctx := context.Background()
	uni := mustProject(t, s, "Uni")
	work := mustProject(t, s, "Work")

	dup := domain.NewProject("uni", 2, now)
	if err := s.CreateProject(ctx, &dup); !errors.Is(err, store.ErrDuplicateName) {
		t.Errorf("duplicate err = %v", err)
	}
	got, err := s.GetProjectByName(ctx, "UNI")
	if err != nil || got.ID != uni.ID {
		t.Errorf("by name = %+v, %v", got, err)
	}
	if _, err := s.GetProject(ctx, domain.ShortID(work.ID)); err != nil {
		t.Errorf("by prefix: %v", err)
	}

	work.Name, work.Icon = "Job", "💼"
	if err := s.UpdateProject(ctx, &work); err != nil {
		t.Fatal(err)
	}

	task := mustCreate(t, s, "essay", func(t *domain.Task) { t.ProjectID = uni.ID })
	if err := s.DeleteProject(ctx, uni.ID, now); err != nil {
		t.Fatal(err)
	}
	ps, _ := s.ListProjects(ctx)
	if len(ps) != 1 || ps[0].Name != "Job" || ps[0].Icon != "💼" {
		t.Errorf("projects = %+v", ps)
	}
	task, _ = s.GetTask(ctx, task.ID)
	if task.ProjectID != "" {
		t.Errorf("task should move to inbox, project=%q", task.ProjectID)
	}
	// The name is free again after deletion.
	again := domain.NewProject("Uni", 3, now)
	if err := s.CreateProject(ctx, &again); err != nil {
		t.Errorf("recreate deleted name: %v", err)
	}
}

// testFiltersAgreeWithDomain checks the backend's native filtering against
// domain.Filter.Match on the same data.
func testFiltersAgreeWithDomain(t *testing.T, s store.Store) {
	ctx := context.Background()
	p := mustProject(t, s, "P")
	day := func(d, h int) *time.Time { return ptr(time.Date(2026, 10, 29+d, h, 0, 0, 0, time.Local)) }
	fixtures := []func(*domain.Task){
		nil,
		func(t *domain.Task) { t.ProjectID = p.ID; t.Tags = []string{"ml"} },
		func(t *domain.Task) { t.Due = day(-2, 0) },
		func(t *domain.Task) { t.Due = day(0, 0) },                      // today, all day: not overdue
		func(t *domain.Task) { t.Due = day(0, 9); t.DueHasTime = true }, // earlier today: overdue
		func(t *domain.Task) { t.Due = day(0, 20); t.DueHasTime = true },
		func(t *domain.Task) { t.Due = day(1, 0); t.ProjectID = p.ID },
		func(t *domain.Task) { t.Due = day(9, 0); t.Tags = []string{"ml", "x"} },
		func(t *domain.Task) { t.Due = day(-1, 0); t.SetStatus(domain.StatusDone, now) },
		func(t *domain.Task) { t.SetStatus(domain.StatusInProgress, now) },
	}
	var all []domain.Task
	for i, mod := range fixtures {
		all = append(all, mustCreate(t, s, "f"+string(rune('a'+i)), mod))
	}
	all = append(all, mustCreate(t, s, "sub", func(t *domain.Task) { t.ParentID = all[0].ID; t.Due = day(0, 0) }))
	deleted := mustCreate(t, s, "deleted", func(t *domain.Task) { t.Due = day(0, 0) })
	if err := s.DeleteTask(ctx, deleted.ID, now); err != nil {
		t.Fatal(err)
	}

	filters := []domain.Filter{{}, {IncludeDone: true}, {IncludeSubtasks: true}, {ProjectID: p.ID}, {Tag: "ML"}, {ParentID: all[0].ID}}
	for _, v := range domain.SmartViews {
		filters = append(filters, domain.Filter{View: v}, domain.Filter{View: v, ProjectID: p.ID})
	}
	for _, f := range filters {
		f.Now = now
		got, err := s.ListTasks(ctx, f, domain.SortManual)
		if err != nil {
			t.Fatal(err)
		}
		var want []string
		for _, task := range all {
			if f.Match(task) {
				want = append(want, task.ID)
			}
		}
		g := ids(got)
		sort.Strings(g)
		sort.Strings(want)
		if len(g) != len(want) {
			t.Errorf("filter %+v: store returned %d tasks, domain matches %d", f, len(g), len(want))
			continue
		}
		for i := range g {
			if g[i] != want[i] {
				t.Errorf("filter %+v: mismatch", f)
				break
			}
		}
		if len(g) == 0 && f.View == domain.ViewToday && f.ProjectID == "" {
			t.Errorf("fixtures should exercise Today")
		}
	}
}

func testSorting(t *testing.T, s store.Store) {
	ctx := context.Background()
	mk := func(title string, prio domain.Priority, due *time.Time, created time.Time) domain.Task {
		task := domain.NewTask(title, created)
		task.Priority, task.Due = prio, due
		if err := s.CreateTask(ctx, &task); err != nil {
			t.Fatal(err)
		}
		return task
	}
	d := func(day int) *time.Time { return ptr(time.Date(2026, 11, day, 0, 0, 0, 0, time.Local)) }
	all := []domain.Task{
		mk("banana", domain.PriorityLow, d(5), now),
		mk("Apple", domain.PriorityUrgent, nil, now.Add(time.Second)),
		mk("cherry", domain.PriorityHigh, d(2), now.Add(2*time.Second)),
		mk("date", domain.PriorityHigh, d(2), now.Add(3*time.Second)),
	}
	for _, k := range domain.SortKeys {
		got, err := s.ListTasks(ctx, domain.Filter{Now: now}, k)
		if err != nil {
			t.Fatal(err)
		}
		want := append([]domain.Task(nil), all...)
		domain.SortTasks(want, k)
		if g, w := ids(got), ids(want); len(g) != len(w) || !equal(g, w) {
			t.Errorf("sort %s: store and domain disagree", k)
		}
	}
}

func equal(a, b []string) bool {
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return len(a) == len(b)
}

func testCountsAndTags(t *testing.T, s store.Store) {
	ctx := context.Background()
	p := mustProject(t, s, "P")
	yesterday := ptr(time.Date(2026, 10, 28, 0, 0, 0, 0, time.Local))
	tonight := ptr(time.Date(2026, 10, 29, 22, 0, 0, 0, time.Local))
	nextWeek := ptr(time.Date(2026, 11, 5, 0, 0, 0, 0, time.Local))

	mustCreate(t, s, "inbox", func(t *domain.Task) { t.Tags = []string{"a"} })
	mustCreate(t, s, "overdue", func(t *domain.Task) { t.Due = yesterday; t.Tags = []string{"a", "b"} })
	mustCreate(t, s, "tonight", func(t *domain.Task) { t.Due = tonight; t.DueHasTime = true; t.ProjectID = p.ID })
	mustCreate(t, s, "later", func(t *domain.Task) { t.Due = nextWeek; t.ProjectID = p.ID })
	mustCreate(t, s, "done today", func(t *domain.Task) { t.Tags = []string{"b"}; t.SetStatus(domain.StatusDone, now) })

	c, err := s.Counts(ctx, now)
	if err != nil {
		t.Fatal(err)
	}
	want := domain.Counts{Inbox: 2, Today: 2, Upcoming: 1, Overdue: 1, Completed: 1, DoneToday: 1}
	if c.Inbox != want.Inbox || c.Today != want.Today || c.Upcoming != want.Upcoming ||
		c.Overdue != want.Overdue || c.Completed != want.Completed || c.DoneToday != want.DoneToday {
		t.Errorf("counts = %+v, want %+v", c, want)
	}
	if c.Projects[p.ID] != 2 {
		t.Errorf("project count = %d", c.Projects[p.ID])
	}
	// Done tasks don't count toward tag badges.
	if c.Tags["a"] != 2 || c.Tags["b"] != 1 {
		t.Errorf("tag counts = %v", c.Tags)
	}
	tags, _ := s.ListTags(ctx)
	if len(tags) != 2 || tags[0].Name != "a" {
		t.Errorf("tags = %+v", tags)
	}
}

func testStats(t *testing.T, s store.Store) {
	ctx := context.Background()
	for i, offset := range []int{0, -1, -1, -2, -5} {
		at := now.AddDate(0, 0, offset)
		mustCreate(t, s, "t"+string(rune('a'+i)), func(t *domain.Task) { t.SetStatus(domain.StatusDone, at) })
	}
	mustCreate(t, s, "open", nil)

	st, err := s.Stats(ctx, now, 7)
	if err != nil {
		t.Fatal(err)
	}
	if st.Streak != 3 || st.Total != 6 || st.Done != 5 || len(st.PerDay) != 7 || st.PerDay[5].Count != 2 {
		t.Errorf("stats = %+v", st)
	}
}

func testChangesSince(t *testing.T, s store.Store) {
	ctx := context.Background()
	old := mustCreate(t, s, "old", nil)
	_ = old
	later := now.Add(time.Hour)
	fresh := domain.NewTask("fresh", later)
	if err := s.CreateTask(ctx, &fresh); err != nil {
		t.Fatal(err)
	}
	p := domain.NewProject("New", 0, later)
	if err := s.CreateProject(ctx, &p); err != nil {
		t.Fatal(err)
	}
	cs, err := s.ChangesSince(ctx, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(cs.Tasks) != 1 || cs.Tasks[0].ID != fresh.ID || len(cs.Projects) != 1 {
		t.Errorf("changes = %d tasks, %d projects", len(cs.Tasks), len(cs.Projects))
	}
}

func testPomodoros(t *testing.T, s store.Store) {
	ctx := context.Background()
	task := mustCreate(t, s, "focus target", nil)
	other := mustCreate(t, s, "other", nil)

	if _, err := s.ActivePomodoro(ctx); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("no active session expected: %v", err)
	}

	// A completed session.
	p1 := domain.NewPomodoro(task.ID, 25*time.Minute, now)
	if err := s.CreatePomodoro(ctx, &p1); err != nil {
		t.Fatal(err)
	}
	// Only one session may run at a time.
	p2 := domain.NewPomodoro(other.ID, 25*time.Minute, now)
	if err := s.CreatePomodoro(ctx, &p2); !errors.Is(err, store.ErrSessionRunning) {
		t.Errorf("second running session err = %v", err)
	}
	// Pause state round-trips through ActivePomodoro.
	p1.Pause(now.Add(5 * time.Minute))
	p1.UpdatedAt = now.Add(5 * time.Minute)
	if err := s.UpdatePomodoro(ctx, &p1); err != nil {
		t.Fatal(err)
	}
	active, err := s.ActivePomodoro(ctx)
	if err != nil || active.ID != p1.ID || !active.IsPaused() || active.Length != 25*time.Minute {
		t.Fatalf("active = %+v, %v", active, err)
	}
	p1.Resume(now.Add(10 * time.Minute))
	p1.Finish(now.Add(40 * time.Minute)) // ran out at +30m (25m + 5m paused)
	if err := s.UpdatePomodoro(ctx, &p1); err != nil {
		t.Fatal(err)
	}

	// An interrupted session: 10 minutes of focus.
	p3 := domain.NewPomodoro(task.ID, 25*time.Minute, now.Add(time.Hour))
	if err := s.CreatePomodoro(ctx, &p3); err != nil {
		t.Fatal(err)
	}
	p3.Finish(now.Add(time.Hour + 10*time.Minute))
	if err := s.UpdatePomodoro(ctx, &p3); err != nil {
		t.Fatal(err)
	}

	got, err := s.GetTask(ctx, task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.PomodorosDone != 1 || got.FocusTime != 35*time.Minute {
		t.Errorf("derived: done=%d focus=%v, want 1 and 35m", got.PomodorosDone, got.FocusTime)
	}
	if _, err := s.ActivePomodoro(ctx); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("all sessions ended, active err = %v", err)
	}
	ps, err := s.ListPomodoros(ctx, now, now.Add(2*time.Hour))
	if err != nil || len(ps) != 2 || !ps[0].Completed || ps[1].Completed {
		t.Errorf("list = %+v, %v", ps, err)
	}
	if ps, _ := s.ListPomodoros(ctx, now.Add(30*time.Minute), now.Add(2*time.Hour)); len(ps) != 1 {
		t.Errorf("range filter: %d", len(ps))
	}

	ghost := domain.NewPomodoro(domain.NewID(), time.Minute, now)
	if err := s.CreatePomodoro(ctx, &ghost); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("session for missing task err = %v", err)
	}
}

func testEstimate(t *testing.T, s store.Store) {
	ctx := context.Background()
	task := mustCreate(t, s, "estimated", func(t *domain.Task) { t.Estimate = 4 })
	got, _ := s.GetTask(ctx, task.ID)
	if got.Estimate != 4 || got.PomodoroProgress() != "0/4" {
		t.Errorf("estimate = %d, progress %q", got.Estimate, got.PomodoroProgress())
	}
	got.Estimate = 6
	if err := s.UpdateTask(ctx, &got); err != nil {
		t.Fatal(err)
	}
	got, _ = s.GetTask(ctx, task.ID)
	if got.Estimate != 6 {
		t.Errorf("updated estimate = %d", got.Estimate)
	}
}
