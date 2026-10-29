package domain

import (
	"strings"
	"time"
)

// DueFilter narrows tasks by due date.
type DueFilter int

// Due filters.
const (
	DueAny DueFilter = iota
	DueSet
	DueNone
	DueOverdue
)

// Refinement is the user's ad-hoc filter on top of the current view
// (priority, due date, status). The zero value matches everything.
type Refinement struct {
	MinPriority Priority
	Due         DueFilter
	Status      Status // "" = any
}

// Active reports whether any constraint is set.
func (r Refinement) Active() bool { return r != Refinement{} }

// Match reports whether t passes the refinement.
func (r Refinement) Match(t Task, now time.Time) bool {
	if t.Priority < r.MinPriority {
		return false
	}
	switch r.Due {
	case DueSet:
		if t.Due == nil {
			return false
		}
	case DueNone:
		if t.Due != nil {
			return false
		}
	case DueOverdue:
		if !t.IsOverdue(now) {
			return false
		}
	}
	return r.Status == "" || t.Status == r.Status
}

// Describe summarizes the active constraints, e.g. "≥high · overdue".
func (r Refinement) Describe() string {
	var parts []string
	if r.MinPriority > PriorityNone {
		s := "≥" + r.MinPriority.String()
		if r.MinPriority == PriorityUrgent {
			s = "urgent"
		}
		parts = append(parts, s)
	}
	switch r.Due {
	case DueSet:
		parts = append(parts, "has due date")
	case DueNone:
		parts = append(parts, "no due date")
	case DueOverdue:
		parts = append(parts, "overdue")
	}
	if r.Status != "" {
		parts = append(parts, r.Status.Label())
	}
	return strings.Join(parts, " · ")
}
