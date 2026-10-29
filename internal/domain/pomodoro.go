package domain

import (
	"errors"
	"fmt"
	"time"
)

// MaxEstimate bounds a task's pomodoro estimate.
const MaxEstimate = 99

// ErrInvalidEstimate is returned for estimates outside 0..MaxEstimate.
var ErrInvalidEstimate = errors.New("estimate must be between 0 and 99 pomodoros")

// Countdown is a pausable timer computed from wall-clock timestamps rather
// than accumulated ticks, so it stays correct across laptop sleep, slow
// renders and app restarts.
type Countdown struct {
	Start    time.Time
	Length   time.Duration
	PausedAt *time.Time    // set while paused
	Paused   time.Duration // total time spent paused before PausedAt
}

// NewCountdown starts a countdown of length at now.
func NewCountdown(length time.Duration, now time.Time) Countdown {
	return Countdown{Start: now, Length: length}
}

// IsPaused reports whether the countdown is paused.
func (c Countdown) IsPaused() bool { return c.PausedAt != nil }

// Elapsed is the running (unpaused) time, capped at Length.
func (c Countdown) Elapsed(now time.Time) time.Duration {
	end := now
	if c.PausedAt != nil {
		end = *c.PausedAt
	}
	e := end.Sub(c.Start) - c.Paused
	return max(0, min(e, c.Length))
}

// Remaining is the time left, never negative.
func (c Countdown) Remaining(now time.Time) time.Duration { return c.Length - c.Elapsed(now) }

// Done reports whether the countdown has run out.
func (c Countdown) Done(now time.Time) bool { return c.Remaining(now) <= 0 }

// Progress is the elapsed fraction in [0,1].
func (c Countdown) Progress(now time.Time) float64 {
	if c.Length <= 0 {
		return 1
	}
	return float64(c.Elapsed(now)) / float64(c.Length)
}

// EndTime is when the countdown finishes (or finished), assuming it is not
// paused from here on.
func (c Countdown) EndTime() time.Time { return c.Start.Add(c.Length + c.Paused) }

// Pause stops the clock; pausing twice is a no-op.
func (c *Countdown) Pause(now time.Time) {
	if c.PausedAt == nil {
		c.PausedAt = &now
	}
}

// Resume restarts the clock, adding the pause to Paused.
func (c *Countdown) Resume(now time.Time) {
	if c.PausedAt != nil {
		c.Paused += now.Sub(*c.PausedAt)
		c.PausedAt = nil
	}
}

// Pomodoro is one focus session on a task. Breaks are not recorded.
type Pomodoro struct {
	ID     string
	TaskID string
	Countdown
	EndedAt   *time.Time // nil while the session is running or paused
	Completed bool       // ran the full length (vs. stopped early)
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time
}

// NewPomodoro starts a focus session on taskID.
func NewPomodoro(taskID string, length time.Duration, now time.Time) Pomodoro {
	return Pomodoro{
		ID: NewID(), TaskID: taskID, Countdown: NewCountdown(length, now),
		CreatedAt: now, UpdatedAt: now,
	}
}

// Running reports whether the session has not ended (it may be paused).
func (p Pomodoro) Running() bool { return p.EndedAt == nil }

// Finish ends the session. If the countdown ran out it is completed and its
// end time is the exact moment it ran out, even when noticed later (e.g.
// the app was closed); otherwise it is recorded as interrupted at now.
func (p *Pomodoro) Finish(now time.Time) {
	if !p.Running() {
		return
	}
	if p.Done(now) {
		p.Completed = true
		end := p.EndTime()
		if p.PausedAt != nil {
			end = *p.PausedAt
		}
		p.EndedAt = &end
	} else {
		p.Pause(now) // freeze elapsed time at now
		p.EndedAt = &now
	}
	p.UpdatedAt = now
}

// Focused is how long was actually spent focusing.
func (p Pomodoro) Focused(now time.Time) time.Duration { return p.Elapsed(now) }

// PomodoroSettings configures the focus/break cycle.
type PomodoroSettings struct {
	Focus      time.Duration
	ShortBreak time.Duration
	LongBreak  time.Duration
	LongEvery  int  // a long break after every N focus sessions
	AutoBreaks bool // start the break automatically after a focus session
	AutoFocus  bool // start the next focus session automatically after a break
}

// DefaultPomodoroSettings is the classic 25/5/15×4 cycle.
func DefaultPomodoroSettings() PomodoroSettings {
	return PomodoroSettings{
		Focus: 25 * time.Minute, ShortBreak: 5 * time.Minute, LongBreak: 15 * time.Minute,
		LongEvery: 4, AutoBreaks: true,
	}
}

// BreakAfter returns the break following the n-th completed focus session
// of a cycle (1-based): a long break every LongEvery sessions.
func (s PomodoroSettings) BreakAfter(n int) (long bool, d time.Duration) {
	if s.LongEvery > 0 && n > 0 && n%s.LongEvery == 0 {
		return true, s.LongBreak
	}
	return false, s.ShortBreak
}

// FormatClock renders a duration as MM:SS (or H:MM:SS past an hour),
// rounding up so a timer never shows 00:00 while time remains.
func FormatClock(d time.Duration) string {
	secs := max(int((d+time.Second-1)/time.Second), 0)
	h, m, sec := secs/3600, secs/60%60, secs%60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, sec)
	}
	return fmt.Sprintf("%02d:%02d", m, sec)
}

// FormatFocus renders total focus time compactly: "<1m", "45m", "2h 05m".
func FormatFocus(d time.Duration) string {
	if d > 0 && d < 30*time.Second {
		return "<1m"
	}
	mins := int(d.Round(time.Minute) / time.Minute)
	if mins < 60 {
		return fmt.Sprintf("%dm", mins)
	}
	return fmt.Sprintf("%dh %02dm", mins/60, mins%60)
}
