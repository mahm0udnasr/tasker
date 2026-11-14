package domain

import (
	"reflect"
	"testing"
	"time"
)

// days builds a done-set from offsets relative to now's date (0 = today,
// -1 = yesterday, …).
func days(offsets ...int) map[string]bool {
	out := map[string]bool{}
	for _, o := range offsets {
		out[DayKey(StartOfDay(now).AddDate(0, 0, o))] = true
	}
	return out
}

func TestHabitStreak(t *testing.T) {
	tests := []struct {
		name string
		done map[string]bool
		want int
	}{
		{"no logs", days(), 0},
		{"done today only", days(0), 1},
		{"done today and the two days before", days(0, -1, -2), 3},
		{"not done today but done yesterday", days(-1, -2), 2},
		{"gap in the middle", days(0, -1, -3, -4, -5), 2},
		{"gap in the middle, not done today", days(-1, -2, -4, -5), 2},
		{"last done two days ago", days(-2, -3), 0},
		{"future days don't count", days(1, 2), 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HabitStreak(tt.done, now); got != tt.want {
				t.Errorf("HabitStreak = %d, want %d", got, tt.want)
			}
		})
	}
}

// Day boundaries are local midnight: a minute before and after midnight
// fall on different days.
func TestHabitStreakDayBoundary(t *testing.T) {
	done := map[string]bool{"2026-10-28": true}
	lateNight := time.Date(2026, 10, 28, 23, 59, 0, 0, time.Local)
	if got := HabitStreak(done, lateNight); got != 1 {
		t.Errorf("same day: streak = %d, want 1", got)
	}
	nextMorning := time.Date(2026, 10, 29, 0, 1, 0, 0, time.Local)
	if got := HabitStreak(done, nextMorning); got != 1 {
		t.Errorf("next day, not done yet: streak = %d, want 1", got)
	}
	dayAfter := time.Date(2026, 10, 30, 0, 1, 0, 0, time.Local)
	if got := HabitStreak(done, dayAfter); got != 0 {
		t.Errorf("a full day missed: streak = %d, want 0", got)
	}
}

// Counting back across a DST change must not skip or repeat a date.
func TestHabitStreakAcrossDST(t *testing.T) {
	loc, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Skip("no tzdata:", err)
	}
	// Clocks went back on Sun 25 Oct 2026 in Berlin.
	at := time.Date(2026, 10, 27, 9, 0, 0, 0, loc)
	done := map[string]bool{"2026-10-27": true, "2026-10-26": true, "2026-10-25": true, "2026-10-24": true}
	if got := HabitStreak(done, at); got != 4 {
		t.Errorf("streak = %d, want 4", got)
	}
}

func TestHabitDays(t *testing.T) {
	got := HabitDays(days(0, -2, -6, -7), now, 7)
	want := []bool{true, false, false, false, true, false, true} // oldest first
	if !reflect.DeepEqual(got, want) {
		t.Errorf("HabitDays = %v, want %v", got, want)
	}
}

func TestHabitValidate(t *testing.T) {
	h := NewHabit("  Read  ", now)
	if err := h.Validate(); err != nil || h.Name != "Read" {
		t.Errorf("Validate = %v, name %q", err, h.Name)
	}
	h.Name = "   "
	if err := h.Validate(); err == nil {
		t.Error("empty name should be rejected")
	}
	if _, err := ParseDay("2026-13-01"); err == nil {
		t.Error("bad day should be rejected")
	}
}
