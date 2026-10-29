package domain

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// SmartView is one of the built-in, rule-based task lists.
type SmartView string

// Smart views shown at the top of the sidebar.
const (
	ViewAll       SmartView = "all" // every open task; used by the CLI default
	ViewInbox     SmartView = "inbox"
	ViewToday     SmartView = "today"
	ViewUpcoming  SmartView = "upcoming"
	ViewOverdue   SmartView = "overdue"
	ViewCompleted SmartView = "completed"
)

// SmartViews lists the views in sidebar order.
var SmartViews = []SmartView{ViewInbox, ViewToday, ViewUpcoming, ViewOverdue, ViewCompleted}

// Title returns the display name.
func (v SmartView) Title() string {
	if v == "" {
		return "All"
	}
	return strings.ToUpper(string(v[:1])) + string(v[1:])
}

// Filter selects tasks. Zero values mean "no constraint"; the View rules are
// combined with ProjectID/Tag using AND.
type Filter struct {
	View      SmartView
	ProjectID string // "" = any project
	Tag       string // normalized tag name; "" = any
	// ParentID lists the subtasks of a task. When empty only top-level tasks
	// are returned, unless IncludeSubtasks is set.
	ParentID        string
	IncludeSubtasks bool
	// IncludeDone also returns completed tasks in views that hide them.
	IncludeDone bool
	// Now anchors the date-based views; the zero value means time.Now().
	Now time.Time
}

// Bounds returns the start of today and of tomorrow, the two instants every
// date-based rule is expressed in.
func (f Filter) Bounds() (startToday, startTomorrow time.Time) {
	now := f.Now
	if now.IsZero() {
		now = time.Now()
	}
	startToday = StartOfDay(now)
	return startToday, startToday.AddDate(0, 0, 1)
}

// Match reports whether t passes the filter. Storage backends that cannot
// express the rules natively can filter in memory with this; the SQLite
// store translates the same rules to SQL and its tests check they agree.
func (f Filter) Match(t Task) bool {
	if t.DeletedAt != nil {
		return false
	}
	switch {
	case f.ParentID != "":
		if t.ParentID != f.ParentID {
			return false
		}
	case !f.IncludeSubtasks && t.ParentID != "":
		return false
	}
	if f.ProjectID != "" && t.ProjectID != f.ProjectID {
		return false
	}
	if f.Tag != "" && !t.HasTag(f.Tag) {
		return false
	}

	now := f.Now
	if now.IsZero() {
		now = time.Now()
	}
	_, tomorrow := f.Bounds()
	openOK := f.IncludeDone || !t.IsDone()

	switch f.View {
	case ViewInbox:
		return openOK && t.ProjectID == ""
	case ViewToday:
		// Today also surfaces everything overdue, Todoist-style.
		return openOK && t.Due != nil && t.Due.Before(tomorrow)
	case ViewUpcoming:
		return openOK && t.Due != nil && !t.Due.Before(tomorrow)
	case ViewOverdue:
		return !t.IsDone() && t.IsOverdue(now)
	case ViewCompleted:
		return t.IsDone()
	default:
		return openOK
	}
}

// SortKey selects list ordering.
type SortKey string

// Sort keys. Every key falls back to manual order, then creation time, so
// ordering is always stable.
const (
	SortManual   SortKey = "manual"
	SortDue      SortKey = "due"
	SortPriority SortKey = "priority"
	SortCreated  SortKey = "created"
	SortTitle    SortKey = "title"
	// SortCompleted (newest completion first) is the Completed view's
	// natural order; it is not part of the cycle in other views.
	SortCompleted SortKey = "completed"
)

// SortKeys lists keys in the order the TUI cycles through them.
var SortKeys = []SortKey{SortManual, SortDue, SortPriority, SortCreated, SortTitle}

// ParseSortKey validates a sort key name.
func ParseSortKey(s string) (SortKey, error) {
	k := SortKey(strings.ToLower(strings.TrimSpace(s)))
	if k == "" {
		return SortManual, nil
	}
	if k == SortCompleted {
		return k, nil
	}
	for _, x := range SortKeys {
		if x == k {
			return k, nil
		}
	}
	return "", fmt.Errorf("unknown sort %q (want manual, due, priority, created, title or completed)", s)
}

// Next cycles to the following sort key.
func (k SortKey) Next() SortKey {
	for i, x := range SortKeys {
		if x == k {
			return SortKeys[(i+1)%len(SortKeys)]
		}
	}
	return SortManual
}

// DefaultSort is the natural ordering for a view.
func DefaultSort(v SmartView) SortKey {
	switch v {
	case ViewToday, ViewUpcoming, ViewOverdue:
		return SortDue
	case ViewCompleted:
		return SortCompleted
	default:
		return SortManual
	}
}

// Less orders two tasks by key. Due dates sort missing-last; priority and
// created sort descending (most important / newest first).
func (k SortKey) Less(a, b Task) bool {
	switch k {
	case SortDue:
		switch {
		case a.Due == nil && b.Due != nil:
			return false
		case a.Due != nil && b.Due == nil:
			return true
		case a.Due != nil && !a.Due.Equal(*b.Due):
			return a.Due.Before(*b.Due)
		}
		if a.Priority != b.Priority {
			return a.Priority > b.Priority
		}
	case SortPriority:
		if a.Priority != b.Priority {
			return a.Priority > b.Priority
		}
	case SortCreated:
		if !a.CreatedAt.Equal(b.CreatedAt) {
			return a.CreatedAt.After(b.CreatedAt)
		}
	case SortCompleted:
		if a.CompletedAt != nil && b.CompletedAt != nil && !a.CompletedAt.Equal(*b.CompletedAt) {
			return a.CompletedAt.After(*b.CompletedAt)
		}
	case SortTitle:
		at, bt := strings.ToLower(a.Title), strings.ToLower(b.Title)
		if at != bt {
			return at < bt
		}
	}
	if a.SortOrder != b.SortOrder {
		return a.SortOrder < b.SortOrder
	}
	return a.CreatedAt.Before(b.CreatedAt)
}

// SortTasks sorts in place by key.
func SortTasks(ts []Task, k SortKey) {
	sort.SliceStable(ts, func(i, j int) bool { return k.Less(ts[i], ts[j]) })
}

// Counts holds the sidebar badge numbers and the inputs for the status-bar
// progress bar.
type Counts struct {
	Inbox     int            `json:"inbox"`
	Today     int            `json:"today"`
	Upcoming  int            `json:"upcoming"`
	Overdue   int            `json:"overdue"`
	Completed int            `json:"completed"`
	Projects  map[string]int `json:"projects"` // open tasks per project ID
	Tags      map[string]int `json:"tags"`     // open tasks per tag name
	// DoneToday counts tasks completed today; together with Today it drives
	// the "today's progress" bar.
	DoneToday int `json:"done_today"`
}

// For returns the badge count for a smart view.
func (c Counts) For(v SmartView) int {
	switch v {
	case ViewInbox:
		return c.Inbox
	case ViewToday:
		return c.Today
	case ViewUpcoming:
		return c.Upcoming
	case ViewOverdue:
		return c.Overdue
	case ViewCompleted:
		return c.Completed
	}
	return 0
}

// TodayProgress returns completed-today / (completed-today + still due today)
// in [0,1], or 0 when there is nothing scheduled.
func (c Counts) TodayProgress() float64 {
	total := c.DoneToday + c.Today
	if total == 0 {
		return 0
	}
	return float64(c.DoneToday) / float64(total)
}
