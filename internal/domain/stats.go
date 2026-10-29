package domain

import "time"

// DayCount is the number of tasks completed on one calendar day.
type DayCount struct {
	Day   time.Time `json:"day"` // local midnight
	Count int       `json:"count"`
}

// Stats summarizes productivity for the stats view.
type Stats struct {
	PerDay         []DayCount `json:"per_day"` // oldest first, ending today
	Streak         int        `json:"streak"`  // consecutive days with ≥1 completion
	BestStreak     int        `json:"best_streak"`
	Total          int        `json:"total"` // live top-level tasks
	Done           int        `json:"done"`
	CompletionRate float64    `json:"completion_rate"` // Done/Total in [0,1]
}

// ComputeStats buckets completion times into the last `days` days and derives
// streaks. total/done describe the whole task set for the completion rate.
//
// The current streak counts back from today, but a day with no completions
// *yet* does not break it: if nothing is done today the streak is measured
// from yesterday, so opening the app in the morning doesn't show 0.
func ComputeStats(completions []time.Time, total, done, days int, now time.Time) Stats {
	today := StartOfDay(now)
	byDay := make(map[time.Time]int, len(completions))
	for _, c := range completions {
		byDay[StartOfDay(c.In(now.Location()))]++
	}

	s := Stats{Total: total, Done: done}
	if total > 0 {
		s.CompletionRate = float64(done) / float64(total)
	}
	for i := days - 1; i >= 0; i-- {
		d := today.AddDate(0, 0, -i)
		s.PerDay = append(s.PerDay, DayCount{Day: d, Count: byDay[d]})
	}

	d := today
	if byDay[d] == 0 {
		d = d.AddDate(0, 0, -1)
	}
	for byDay[d] > 0 {
		s.Streak++
		d = d.AddDate(0, 0, -1)
	}

	// Best streak: walk the distinct days in order.
	if len(byDay) > 0 {
		first := today
		for day := range byDay {
			if day.Before(first) {
				first = day
			}
		}
		run := 0
		for day := first; !day.After(today); day = day.AddDate(0, 0, 1) {
			if byDay[day] > 0 {
				run++
				if run > s.BestStreak {
					s.BestStreak = run
				}
			} else {
				run = 0
			}
		}
	}
	return s
}

// FocusDay is the focus time and completed pomodoros of one calendar day.
type FocusDay struct {
	Day       time.Time     `json:"day"`
	Focus     time.Duration `json:"-"`
	Pomodoros int           `json:"pomodoros"`
}

// FocusStats summarizes pomodoro sessions over a window of days.
type FocusStats struct {
	PerDay    []FocusDay    `json:"per_day"` // oldest first, ending today
	Total     time.Duration `json:"-"`
	Pomodoros int           `json:"pomodoros"`
}

// ComputeFocus buckets finished sessions by the day they started, over the
// last `days` days. Running sessions are ignored.
func ComputeFocus(sessions []Pomodoro, days int, now time.Time) FocusStats {
	today := StartOfDay(now)
	idx := make(map[time.Time]int, days)
	fs := FocusStats{PerDay: make([]FocusDay, days)}
	for i := range days {
		d := today.AddDate(0, 0, i-days+1)
		fs.PerDay[i].Day = d
		idx[d] = i
	}
	for _, p := range sessions {
		if p.Running() || p.DeletedAt != nil {
			continue
		}
		i, ok := idx[StartOfDay(p.Start.In(now.Location()))]
		if !ok {
			continue
		}
		f := p.Focused(now)
		fs.PerDay[i].Focus += f
		fs.Total += f
		if p.Completed {
			fs.PerDay[i].Pomodoros++
			fs.Pomodoros++
		}
	}
	return fs
}
