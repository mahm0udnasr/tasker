package domain

import (
	"fmt"
	"strings"
	"time"
)

// MaxHabitNameLen bounds habit names, like project names.
const MaxHabitNameLen = 100

// DayLayout is how habit logs record a calendar day. Logs store the local
// date rather than an instant, so "done on Tuesday" stays Tuesday no matter
// when (or in which zone) it is read back.
const DayLayout = "2006-01-02"

// Habit is something done daily. Unlike a task it is never completed; its
// history lives in HabitLog entries, and streaks are derived from them.
type Habit struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `json:"deleted_at,omitempty"`
}

// HabitLog records that a habit was done on Day (DayLayout, local date).
// Undoing a day's completion tombstones the entry with DeletedAt.
type HabitLog struct {
	HabitID   string     `json:"habit_id"`
	Day       string     `json:"day"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `json:"deleted_at,omitempty"`
}

// NewHabit builds a habit created at now.
func NewHabit(name string, now time.Time) Habit {
	return Habit{ID: NewID(), Name: strings.TrimSpace(name), CreatedAt: now, UpdatedAt: now}
}

// Validate checks habit invariants.
func (h *Habit) Validate() error {
	h.Name = strings.TrimSpace(h.Name)
	switch {
	case h.Name == "":
		return fmt.Errorf("habit name: %w", ErrEmptyTitle)
	case len([]rune(h.Name)) > MaxHabitNameLen:
		return fmt.Errorf("habit name must be at most %d characters", MaxHabitNameLen)
	}
	return nil
}

// DayKey returns t's calendar date in t's own location, in DayLayout.
// Callers pass local times, so day boundaries are local midnight.
func DayKey(t time.Time) string { return t.Format(DayLayout) }

// ParseDay validates a DayLayout date.
func ParseDay(s string) (time.Time, error) {
	d, err := time.ParseInLocation(DayLayout, s, time.Local)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid day %q (want YYYY-MM-DD)", s)
	}
	return d, nil
}

// HabitStreak counts consecutive done days ending today, or ending
// yesterday when today isn't done yet: an unfinished today doesn't break
// the streak until the day is over. done is keyed by DayKey.
func HabitStreak(done map[string]bool, now time.Time) int {
	day := StartOfDay(now)
	if !done[DayKey(day)] {
		day = day.AddDate(0, 0, -1)
	}
	n := 0
	for done[DayKey(day)] {
		n++
		day = day.AddDate(0, 0, -1)
	}
	return n
}

// HabitDays reports, oldest first, whether each of the last n days (ending
// today) is done.
func HabitDays(done map[string]bool, now time.Time, n int) []bool {
	out := make([]bool, n)
	today := StartOfDay(now)
	for i := range out {
		out[i] = done[DayKey(today.AddDate(0, 0, i-n+1))]
	}
	return out
}
