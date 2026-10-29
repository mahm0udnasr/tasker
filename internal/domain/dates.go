package domain

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"strings"
	"time"

	"github.com/olebedev/when"
	"github.com/olebedev/when/rules/common"
	"github.com/olebedev/when/rules/en"
)

// ErrUnparsableDate is returned when a due-date string cannot be understood.
var ErrUnparsableDate = errors.New("could not understand date")

var nlParser = func() *when.Parser {
	p := when.New(nil)
	p.Add(en.All...)
	p.Add(common.All...)
	return p
}()

// Formats tried before natural-language parsing, in order. Layouts with a
// clock component set hasTime.
var exactLayouts = []struct {
	layout  string
	hasTime bool
}{
	{"2006-01-02 15:04", true},
	{"2006-01-02T15:04", true},
	{"2006-01-02 3:04pm", true},
	{"2006-01-02 3pm", true},
	{"2006-01-02", false},
}

// ParseDue turns user input such as "tomorrow 5pm", "next friday",
// "in 3 days" or "2026-11-02 09:30" into a due time. A nil time with a nil
// error means the input asked to clear the date ("", "none", "clear").
// hasTime is false for whole-day dates, which are normalized to local midnight.
func ParseDue(input string, now time.Time) (due *time.Time, hasTime bool, err error) {
	s := strings.ToLower(strings.TrimSpace(input))
	switch s {
	case "", "none", "no", "clear", "never", "-":
		return nil, false, nil
	case "eod":
		t := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 0, 0, now.Location())
		return &t, true, nil
	case "next week": // Monday of next week
		d := StartOfDay(now)
		offset := (8 - int(d.Weekday())) % 7
		if offset == 0 {
			offset = 7
		}
		return finish(d.AddDate(0, 0, offset), false)
	case "weekend", "this weekend": // the coming Saturday (today if Saturday)
		d := StartOfDay(now)
		return finish(d.AddDate(0, 0, (int(time.Saturday)-int(d.Weekday())+7)%7), false)
	case "next month": // first day of next month
		return finish(time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, now.Location()), false)
	}

	for _, l := range exactLayouts {
		if t, perr := time.ParseInLocation(l.layout, s, now.Location()); perr == nil {
			return finish(t, l.hasTime)
		}
	}

	// The parser has no notion of "this phrase specified a time of day", and
	// it moves across days by adding 24h, which drifts by an hour over a DST
	// change. So: detect a time-of-day ourselves, and for date-only phrases
	// parse from local noon, where a ±1h drift can never change the date.
	hasTime = timeOfDayRe.MatchString(s)
	base := now
	if !hasTime {
		base = StartOfDay(now).Add(12 * time.Hour)
	}
	res, perr := nlParser.Parse(s, base)
	if perr != nil || res == nil {
		return nil, false, fmt.Errorf("%w: %q", ErrUnparsableDate, input)
	}
	return finish(res.Time, hasTime)
}

// timeOfDayRe recognizes phrases that pin a clock time: "5pm", "17:30",
// "noon", "tonight", "in 2 hours", "this evening".
var timeOfDayRe = regexp.MustCompile(`\d\s*(am|pm)\b|\d:\d\d|\b(noon|midnight|tonight|morning|afternoon|evening)\b|\b(hours?|hrs?|minutes?|mins?)\b`)

func finish(t time.Time, hasTime bool) (*time.Time, bool, error) {
	if hasTime {
		t = t.Truncate(time.Minute)
	} else {
		t = StartOfDay(t)
	}
	return &t, hasTime, nil
}

// StartOfDay returns local midnight of t's day.
func StartOfDay(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

// SameDay reports whether a and b fall on the same calendar day in b's zone.
func SameDay(a, b time.Time) bool {
	a = a.In(b.Location())
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
}

// DaysBetween counts calendar days from a to b (negative if b is earlier).
func DaysBetween(a, b time.Time) int {
	a, b = StartOfDay(a), StartOfDay(b.In(a.Location()))
	// Round so DST transitions (23h/25h days) still count as whole days.
	return int(math.Round(b.Sub(a).Hours() / 24))
}

// FormatDue renders a due date relative to now: "today 17:00", "tomorrow",
// "yesterday", "Fri", "Mon 3 Nov", "3 Nov 2027".
func FormatDue(due time.Time, hasTime bool, now time.Time) string {
	due = due.In(now.Location())
	var day string
	switch d := DaysBetween(now, due); {
	case d == 0:
		day = "today"
	case d == 1:
		day = "tomorrow"
	case d == -1:
		day = "yesterday"
	case d > 1 && d < 7:
		day = due.Format("Mon")
	case due.Year() == now.Year():
		day = due.Format("Mon 2 Jan")
	default:
		day = due.Format("2 Jan 2006")
	}
	if hasTime {
		return day + " " + due.Format("15:04")
	}
	return day
}
