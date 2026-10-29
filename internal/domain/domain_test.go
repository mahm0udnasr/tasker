package domain

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

// Thursday 29 Oct 2026, 14:30:15 local.
var now = time.Date(2026, 10, 29, 14, 30, 15, 0, time.Local)

func at(y int, m time.Month, d, h, min int) *time.Time {
	t := time.Date(y, m, d, h, min, 0, 0, time.Local)
	return &t
}

func TestParsePriority(t *testing.T) {
	tests := []struct {
		in      string
		want    Priority
		wantErr bool
	}{
		{"", PriorityNone, false},
		{"none", PriorityNone, false},
		{"LOW", PriorityLow, false},
		{"med", PriorityMedium, false},
		{" high ", PriorityHigh, false},
		{"urgent", PriorityUrgent, false},
		{"p1", PriorityUrgent, false},
		{"4", PriorityUrgent, false},
		{"critical", PriorityNone, true},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, err := ParsePriority(tt.in)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPriorityNextCycles(t *testing.T) {
	p := PriorityNone
	seen := []Priority{}
	for range 6 {
		p = p.Next()
		seen = append(seen, p)
	}
	want := []Priority{PriorityLow, PriorityMedium, PriorityHigh, PriorityUrgent, PriorityNone, PriorityLow}
	if !reflect.DeepEqual(seen, want) {
		t.Errorf("got %v, want %v", seen, want)
	}
}

func TestParseStatus(t *testing.T) {
	tests := []struct {
		in   string
		want Status
		err  bool
	}{
		{"todo", StatusTodo, false},
		{"doing", StatusInProgress, false},
		{"in-progress", StatusInProgress, false},
		{"DONE", StatusDone, false},
		{"blocked", "", true},
	}
	for _, tt := range tests {
		got, err := ParseStatus(tt.in)
		if (err != nil) != tt.err || got != tt.want {
			t.Errorf("ParseStatus(%q) = %q, %v", tt.in, got, err)
		}
	}
}

func TestTaskValidate(t *testing.T) {
	tests := []struct {
		name string
		mod  func(*Task)
		want error
	}{
		{"ok", func(*Task) {}, nil},
		{"blank title", func(t *Task) { t.Title = "   " }, ErrEmptyTitle},
		{"long title", func(t *Task) { t.Title = strings.Repeat("x", MaxTitleLen+1) }, ErrTitleTooLong},
		{"bad status", func(t *Task) { t.Status = "nope" }, ErrInvalidStatus},
		{"bad priority", func(t *Task) { t.Priority = 9 }, ErrInvalidPrio},
		{"self parent", func(t *Task) { t.ParentID = t.ID }, ErrSelfParent},
		{"bad tag", func(t *Task) { t.Tags = []string{"#"} }, ErrInvalidTagName},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := NewTask("Write report", now)
			tt.mod(&task)
			err := task.Validate()
			if !errors.Is(err, tt.want) {
				t.Errorf("Validate() = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestSetStatusMaintainsCompletedAt(t *testing.T) {
	task := NewTask("x", now)
	later := now.Add(time.Hour)

	task.ToggleDone(later)
	if task.Status != StatusDone || task.CompletedAt == nil || !task.CompletedAt.Equal(later) {
		t.Fatalf("after toggle: status=%s completed=%v", task.Status, task.CompletedAt)
	}
	if !task.UpdatedAt.Equal(later) {
		t.Errorf("UpdatedAt not bumped")
	}
	task.ToggleDone(later)
	if task.Status != StatusTodo || task.CompletedAt != nil {
		t.Fatalf("after second toggle: status=%s completed=%v", task.Status, task.CompletedAt)
	}
	task.SetStatus(StatusInProgress, later)
	if task.CompletedAt != nil {
		t.Errorf("in_progress must not have CompletedAt")
	}
}

func TestNormalizeTags(t *testing.T) {
	tests := []struct {
		in   []string
		want []string
		err  bool
	}{
		{[]string{"#ML", "ml", " Machine  Learning "}, []string{"machine-learning", "ml"}, false},
		{[]string{"a,b", "b"}, []string{"a", "b"}, false},
		{[]string{"", " , "}, nil, false},
		{[]string{"#"}, nil, true},
	}
	for _, tt := range tests {
		got, err := NormalizeTags(tt.in)
		if (err != nil) != tt.err || !reflect.DeepEqual(got, tt.want) {
			t.Errorf("NormalizeTags(%q) = %q, %v; want %q", tt.in, got, err, tt.want)
		}
	}
}

func TestProjectValidate(t *testing.T) {
	p := NewProject("  Uni ", 0, now)
	if err := p.Validate(); err != nil || p.Name != "Uni" {
		t.Fatalf("got %v name=%q", err, p.Name)
	}
	p.Color = "blue"
	if err := p.Validate(); err == nil {
		t.Errorf("expected color error")
	}
}

func TestParseDue(t *testing.T) {
	tests := []struct {
		in       string
		want     *time.Time
		wantTime bool
		wantErr  bool
	}{
		{"", nil, false, false},
		{"none", nil, false, false},
		{"tomorrow", at(2026, 10, 30, 0, 0), false, false},
		{"tomorrow 8pm", at(2026, 10, 30, 20, 0), true, false},
		{"today 5pm", at(2026, 10, 29, 17, 0), true, false},
		{"friday", at(2026, 10, 30, 0, 0), false, false},
		{"next monday", at(2026, 11, 2, 0, 0), false, false},
		{"next week", at(2026, 11, 2, 0, 0), false, false},
		{"weekend", at(2026, 10, 31, 0, 0), false, false},
		{"next month", at(2026, 11, 1, 0, 0), false, false},
		{"tonight", at(2026, 10, 29, 23, 0), true, false},
		{"in 3 days", at(2026, 11, 1, 0, 0), false, false},
		{"in 2 hours", at(2026, 10, 29, 16, 30), true, false},
		{"2026-12-24", at(2026, 12, 24, 0, 0), false, false},
		{"2026-12-24 09:15", at(2026, 12, 24, 9, 15), true, false},
		{"eod", at(2026, 10, 29, 23, 59), true, false},
		{"gibberish words", nil, false, true},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, hasTime, err := ParseDue(tt.in, now)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.want == nil {
				if got != nil {
					t.Fatalf("got %v, want nil", got)
				}
				return
			}
			if got == nil || !got.Equal(*tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
			if hasTime != tt.wantTime {
				t.Errorf("hasTime = %v, want %v", hasTime, tt.wantTime)
			}
		})
	}
}

func TestFormatDue(t *testing.T) {
	tests := []struct {
		due     *time.Time
		hasTime bool
		want    string
	}{
		{at(2026, 10, 29, 17, 0), true, "today 17:00"},
		{at(2026, 10, 30, 0, 0), false, "tomorrow"},
		{at(2026, 10, 28, 0, 0), false, "yesterday"},
		{at(2026, 11, 2, 0, 0), false, "Mon"},
		{at(2026, 11, 20, 0, 0), false, "Fri 20 Nov"},
		{at(2027, 1, 3, 0, 0), false, "3 Jan 2027"},
	}
	for _, tt := range tests {
		if got := FormatDue(*tt.due, tt.hasTime, now); got != tt.want {
			t.Errorf("FormatDue(%v) = %q, want %q", tt.due, got, tt.want)
		}
	}
}

func TestIsOverdue(t *testing.T) {
	tests := []struct {
		name    string
		due     *time.Time
		hasTime bool
		done    bool
		want    bool
	}{
		{"no due", nil, false, false, false},
		{"earlier today timed", at(2026, 10, 29, 9, 0), true, false, true},
		{"later today timed", at(2026, 10, 29, 18, 0), true, false, false},
		{"today all-day", at(2026, 10, 29, 0, 0), false, false, false},
		{"yesterday all-day", at(2026, 10, 28, 0, 0), false, false, true},
		{"done never overdue", at(2026, 10, 1, 0, 0), false, true, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := NewTask("x", now)
			task.SetDue(tt.due, tt.hasTime, now)
			if tt.done {
				task.SetStatus(StatusDone, now)
			}
			if got := task.IsOverdue(now); got != tt.want {
				t.Errorf("IsOverdue = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFilterMatch(t *testing.T) {
	mk := func(mod func(*Task)) Task {
		task := NewTask("x", now)
		mod(&task)
		return task
	}
	inbox := mk(func(*Task) {})
	project := mk(func(t *Task) { t.ProjectID = "p1"; t.Tags = []string{"ml"} })
	overdue := mk(func(t *Task) { t.Due = at(2026, 10, 27, 0, 0) })
	today := mk(func(t *Task) { t.Due = at(2026, 10, 29, 20, 0); t.DueHasTime = true })
	future := mk(func(t *Task) { t.Due = at(2026, 11, 3, 0, 0) })
	done := mk(func(t *Task) { t.SetStatus(StatusDone, now) })
	sub := mk(func(t *Task) { t.ParentID = "parent" })
	deleted := mk(func(t *Task) { t.DeletedAt = &now })

	all := []Task{inbox, project, overdue, today, future, done, sub, deleted}
	tests := []struct {
		name   string
		filter Filter
		want   []Task
	}{
		{"all open", Filter{}, []Task{inbox, project, overdue, today, future}},
		{"inbox", Filter{View: ViewInbox}, []Task{inbox, overdue, today, future}},
		{"today includes overdue", Filter{View: ViewToday}, []Task{overdue, today}},
		{"upcoming", Filter{View: ViewUpcoming}, []Task{future}},
		{"overdue", Filter{View: ViewOverdue}, []Task{overdue}},
		{"completed", Filter{View: ViewCompleted}, []Task{done}},
		{"project", Filter{ProjectID: "p1"}, []Task{project}},
		{"tag", Filter{Tag: "#ML"}, []Task{project}},
		{"subtasks of parent", Filter{ParentID: "parent"}, []Task{sub}},
		{"include subtasks+done", Filter{IncludeSubtasks: true, IncludeDone: true}, []Task{inbox, project, overdue, today, future, done, sub}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.filter.Now = now
			var got []Task
			for _, task := range all {
				if tt.filter.Match(task) {
					got = append(got, task)
				}
			}
			if !sameIDs(got, tt.want) {
				t.Errorf("got %d tasks, want %d", len(got), len(tt.want))
			}
		})
	}
}

func sameIDs(a, b []Task) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].ID != b[i].ID {
			return false
		}
	}
	return true
}

func TestSortTasks(t *testing.T) {
	a := NewTask("banana", now)
	a.Priority = PriorityLow
	a.Due = at(2026, 11, 2, 0, 0)
	a.SortOrder = 3
	b := NewTask("Apple", now.Add(time.Second))
	b.Priority = PriorityUrgent
	b.SortOrder = 1
	c := NewTask("cherry", now.Add(2*time.Second))
	c.Priority = PriorityHigh
	c.Due = at(2026, 10, 30, 0, 0)
	c.SortOrder = 2

	tests := []struct {
		key  SortKey
		want []string
	}{
		{SortManual, []string{"Apple", "cherry", "banana"}},
		{SortDue, []string{"cherry", "banana", "Apple"}},
		{SortPriority, []string{"Apple", "cherry", "banana"}},
		{SortCreated, []string{"cherry", "Apple", "banana"}},
		{SortTitle, []string{"Apple", "banana", "cherry"}},
	}
	for _, tt := range tests {
		ts := []Task{a, b, c}
		SortTasks(ts, tt.key)
		var got []string
		for _, x := range ts {
			got = append(got, x.Title)
		}
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%s: got %v, want %v", tt.key, got, tt.want)
		}
	}
}

func TestComputeStats(t *testing.T) {
	day := func(offset, hour int) time.Time {
		return time.Date(2026, 10, 29+offset, hour, 0, 0, 0, time.Local)
	}
	tests := []struct {
		name        string
		completions []time.Time
		wantStreak  int
		wantBest    int
		wantToday   int
	}{
		{"none", nil, 0, 0, 0},
		{"today only", []time.Time{day(0, 9)}, 1, 1, 1},
		{"yesterday keeps streak alive", []time.Time{day(-1, 9), day(-2, 9)}, 2, 2, 0},
		{"gap breaks streak", []time.Time{day(0, 9), day(-1, 9), day(-3, 9), day(-4, 9), day(-5, 9)}, 2, 3, 1},
		{"two days ago is broken", []time.Time{day(-2, 9)}, 0, 1, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := ComputeStats(tt.completions, 10, len(tt.completions), 7, now)
			if s.Streak != tt.wantStreak || s.BestStreak != tt.wantBest {
				t.Errorf("streak=%d best=%d, want %d/%d", s.Streak, s.BestStreak, tt.wantStreak, tt.wantBest)
			}
			if len(s.PerDay) != 7 || s.PerDay[6].Count != tt.wantToday {
				t.Errorf("per-day = %+v", s.PerDay)
			}
		})
	}
}

func TestCountsTodayProgress(t *testing.T) {
	if p := (Counts{Today: 3, DoneToday: 1}).TodayProgress(); p != 0.25 {
		t.Errorf("progress = %v", p)
	}
	if p := (Counts{}).TodayProgress(); p != 0 {
		t.Errorf("empty progress = %v", p)
	}
}

func TestRefinement(t *testing.T) {
	high := NewTask("high", now)
	high.Priority = PriorityHigh
	high.Due = at(2026, 10, 27, 0, 0) // overdue
	low := NewTask("low", now)
	low.Priority = PriorityLow
	low.SetStatus(StatusInProgress, now)
	urgent := NewTask("urgent", now)
	urgent.Priority = PriorityUrgent
	urgent.Due = at(2026, 11, 2, 0, 0)

	tests := []struct {
		name string
		r    Refinement
		want []string
		desc string
	}{
		{"zero matches all", Refinement{}, []string{"high", "low", "urgent"}, ""},
		{"min priority", Refinement{MinPriority: PriorityHigh}, []string{"high", "urgent"}, "≥high"},
		{"urgent only", Refinement{MinPriority: PriorityUrgent}, []string{"urgent"}, "urgent"},
		{"has due", Refinement{Due: DueSet}, []string{"high", "urgent"}, "has due date"},
		{"no due", Refinement{Due: DueNone}, []string{"low"}, "no due date"},
		{"overdue", Refinement{Due: DueOverdue}, []string{"high"}, "overdue"},
		{"status", Refinement{Status: StatusInProgress}, []string{"low"}, "in progress"},
		{"combined", Refinement{MinPriority: PriorityHigh, Due: DueSet}, []string{"high", "urgent"}, "≥high · has due date"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got []string
			for _, task := range []Task{high, low, urgent} {
				if tt.r.Match(task, now) {
					got = append(got, task.Title)
				}
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
			if d := tt.r.Describe(); d != tt.desc {
				t.Errorf("Describe() = %q, want %q", d, tt.desc)
			}
			if tt.r.Active() != (tt.desc != "") {
				t.Errorf("Active() = %v", tt.r.Active())
			}
		})
	}
}

func TestRelative(t *testing.T) {
	tests := []struct {
		due     *time.Time
		hasTime bool
		want    string
	}{
		{at(2026, 10, 29, 17, 30), true, "in 2h"},
		{at(2026, 10, 29, 14, 0), true, "30m ago"},
		{at(2026, 10, 29, 0, 0), false, "today"},
		{at(2026, 10, 30, 0, 0), false, "tomorrow"},
		{at(2026, 10, 28, 0, 0), false, "yesterday"},
		{at(2026, 11, 3, 0, 0), false, "in 5 days"},
		{at(2026, 10, 25, 0, 0), false, "4 days ago"},
		{at(2026, 11, 2, 9, 0), true, "in 4 days"},
	}
	for _, tt := range tests {
		if got := Relative(*tt.due, tt.hasTime, now); got != tt.want {
			t.Errorf("Relative(%v) = %q, want %q", tt.due, got, tt.want)
		}
	}
}
