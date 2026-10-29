// Package domain holds tasker's core types and business rules. It performs no
// I/O, so everything here is deterministic and cheap to test; callers inject
// the current time instead of the package reading the clock.
package domain

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// MaxTitleLen bounds task and project titles so a pasted blob cannot wreck
// the list layout.
const MaxTitleLen = 500

// Validation errors returned by Validate methods.
var (
	ErrEmptyTitle     = errors.New("title must not be empty")
	ErrTitleTooLong   = fmt.Errorf("title must be at most %d characters", MaxTitleLen)
	ErrInvalidStatus  = errors.New("invalid status")
	ErrInvalidPrio    = errors.New("invalid priority")
	ErrSelfParent     = errors.New("a task cannot be its own parent")
	ErrNestedSubtask  = errors.New("subtasks cannot have subtasks of their own")
	ErrInvalidTagName = errors.New("invalid tag name")
)

// Status is the workflow state of a task.
type Status string

// Task statuses.
const (
	StatusTodo       Status = "todo"
	StatusInProgress Status = "in_progress"
	StatusDone       Status = "done"
)

// Statuses lists every valid status in workflow order.
var Statuses = []Status{StatusTodo, StatusInProgress, StatusDone}

// Valid reports whether s is a known status.
func (s Status) Valid() bool {
	switch s {
	case StatusTodo, StatusInProgress, StatusDone:
		return true
	}
	return false
}

// Label returns a human-friendly name.
func (s Status) Label() string {
	switch s {
	case StatusInProgress:
		return "in progress"
	default:
		return string(s)
	}
}

// ParseStatus accepts the canonical names plus a few forgiving aliases.
func ParseStatus(in string) (Status, error) {
	switch strings.ToLower(strings.TrimSpace(in)) {
	case "todo", "open", "":
		return StatusTodo, nil
	case "in_progress", "in-progress", "inprogress", "doing", "wip", "progress":
		return StatusInProgress, nil
	case "done", "complete", "completed", "closed":
		return StatusDone, nil
	}
	return "", fmt.Errorf("%w: %q (want todo, in_progress or done)", ErrInvalidStatus, in)
}

// Priority orders tasks by importance. The zero value means "no priority".
type Priority int

// Task priorities, lowest to highest.
const (
	PriorityNone Priority = iota
	PriorityLow
	PriorityMedium
	PriorityHigh
	PriorityUrgent
)

var priorityNames = [...]string{"none", "low", "medium", "high", "urgent"}

// Valid reports whether p is within range.
func (p Priority) Valid() bool { return p >= PriorityNone && p <= PriorityUrgent }

// String implements fmt.Stringer.
func (p Priority) String() string {
	if !p.Valid() {
		return fmt.Sprintf("Priority(%d)", int(p))
	}
	return priorityNames[p]
}

// Next cycles none → low → medium → high → urgent → none.
func (p Priority) Next() Priority { return (p + 1) % (PriorityUrgent + 1) }

// ParsePriority accepts names, prefixes ("med", "u"), digits 0-4 and the
// Todoist-style p1..p4 (where p1 is the most urgent).
func ParsePriority(in string) (Priority, error) {
	s := strings.ToLower(strings.TrimSpace(in))
	switch s {
	case "", "none", "no", "0", "-":
		return PriorityNone, nil
	case "l", "low", "1", "p4":
		return PriorityLow, nil
	case "m", "med", "medium", "2", "p3":
		return PriorityMedium, nil
	case "h", "hi", "high", "3", "p2":
		return PriorityHigh, nil
	case "u", "urgent", "!", "4", "p1":
		return PriorityUrgent, nil
	}
	return PriorityNone, fmt.Errorf("%w: %q (want none, low, medium, high or urgent)", ErrInvalidPrio, in)
}

// Task is a single to-do item. Subtasks are Tasks whose ParentID is set; only
// one level of nesting is allowed.
type Task struct {
	ID          string     `json:"id"`
	ParentID    string     `json:"parent_id,omitempty"`
	ProjectID   string     `json:"project_id,omitempty"`
	Title       string     `json:"title"`
	Description string     `json:"description,omitempty"`
	Status      Status     `json:"status"`
	Priority    Priority   `json:"priority"`
	Due         *time.Time `json:"due,omitempty"`
	// DueHasTime distinguishes "friday" (all day; Due is local midnight)
	// from "friday 5pm".
	DueHasTime bool     `json:"due_has_time,omitempty"`
	Tags       []string `json:"tags,omitempty"`
	// Estimate is the planned number of pomodoros (0 = no estimate).
	Estimate    int        `json:"estimate,omitempty"`
	SortOrder   float64    `json:"sort_order"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	DeletedAt   *time.Time `json:"deleted_at,omitempty"`

	// Derived fields filled in by the store on reads; never persisted.
	SubtasksTotal int `json:"subtasks_total,omitempty"`
	SubtasksDone  int `json:"subtasks_done,omitempty"`
	// PomodorosDone counts completed focus sessions; FocusTime sums all
	// focus time, including sessions stopped early.
	PomodorosDone int           `json:"pomodoros_done,omitempty"`
	FocusTime     time.Duration `json:"-"`
}

// NewID returns a fresh random identifier. Random (v4) rather than
// time-ordered UUIDs are used so that short prefixes shown in the CLI are
// unique even for tasks created in the same second.
func NewID() string { return uuid.NewString() }

// ShortID is the prefix shown to users; the store resolves any unique prefix.
func ShortID(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

// NewTask builds a valid todo task with timestamps set to now.
func NewTask(title string, now time.Time) Task {
	return Task{
		ID:        NewID(),
		Title:     strings.TrimSpace(title),
		Status:    StatusTodo,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// Validate checks invariants that hold regardless of storage.
func (t *Task) Validate() error {
	t.Title = strings.TrimSpace(t.Title)
	switch {
	case t.Title == "":
		return ErrEmptyTitle
	case len([]rune(t.Title)) > MaxTitleLen:
		return ErrTitleTooLong
	case !t.Status.Valid():
		return fmt.Errorf("%w: %q", ErrInvalidStatus, t.Status)
	case !t.Priority.Valid():
		return fmt.Errorf("%w: %d", ErrInvalidPrio, t.Priority)
	case t.ParentID != "" && t.ParentID == t.ID:
		return ErrSelfParent
	case t.Estimate < 0 || t.Estimate > MaxEstimate:
		return ErrInvalidEstimate
	}
	for _, tag := range t.Tags {
		if _, err := NormalizeTag(tag); err != nil {
			return err
		}
	}
	return nil
}

// IsDone reports whether the task is completed.
func (t Task) IsDone() bool { return t.Status == StatusDone }

// IsSubtask reports whether the task has a parent.
func (t Task) IsSubtask() bool { return t.ParentID != "" }

// SetStatus changes the status and keeps CompletedAt consistent with it.
func (t *Task) SetStatus(s Status, now time.Time) {
	if t.Status == s {
		return
	}
	t.Status = s
	if s == StatusDone {
		t.CompletedAt = &now
	} else {
		t.CompletedAt = nil
	}
	t.UpdatedAt = now
}

// ToggleDone flips between done and todo.
func (t *Task) ToggleDone(now time.Time) {
	if t.IsDone() {
		t.SetStatus(StatusTodo, now)
	} else {
		t.SetStatus(StatusDone, now)
	}
}

// CyclePriority advances to the next priority level.
func (t *Task) CyclePriority(now time.Time) {
	t.Priority = t.Priority.Next()
	t.UpdatedAt = now
}

// SetDue sets or clears (nil) the due date.
func (t *Task) SetDue(due *time.Time, hasTime bool, now time.Time) {
	t.Due = due
	t.DueHasTime = due != nil && hasTime
	t.UpdatedAt = now
}

// SetTags replaces the tag set with normalized, de-duplicated names.
func (t *Task) SetTags(tags []string) error {
	norm, err := NormalizeTags(tags)
	if err != nil {
		return err
	}
	t.Tags = norm
	return nil
}

// HasTag reports whether the task carries the (normalized) tag.
func (t Task) HasTag(tag string) bool {
	n, err := NormalizeTag(tag)
	if err != nil {
		return false
	}
	for _, x := range t.Tags {
		if x == n {
			return true
		}
	}
	return false
}

// IsOverdue reports whether an open task is past due. All-day tasks become
// overdue only once their day has ended.
func (t Task) IsOverdue(now time.Time) bool {
	if t.Due == nil || t.IsDone() {
		return false
	}
	if t.DueHasTime {
		return t.Due.Before(now)
	}
	return t.Due.Before(StartOfDay(now))
}

// IsDueToday reports whether the due date falls on now's calendar day.
func (t Task) IsDueToday(now time.Time) bool {
	return t.Due != nil && SameDay(*t.Due, now)
}

// PomodoroProgress returns "3/4" with an estimate, "3" without, or "" when
// there is neither.
func (t Task) PomodoroProgress() string {
	switch {
	case t.Estimate > 0:
		return fmt.Sprintf("%d/%d", t.PomodorosDone, t.Estimate)
	case t.PomodorosDone > 0:
		return fmt.Sprintf("%d", t.PomodorosDone)
	}
	return ""
}

// SubtaskProgress returns "done/total" or "" when the task has no subtasks.
func (t Task) SubtaskProgress() string {
	if t.SubtasksTotal == 0 {
		return ""
	}
	return fmt.Sprintf("%d/%d", t.SubtasksDone, t.SubtasksTotal)
}
