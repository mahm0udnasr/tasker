package service

import (
	"context"
	"time"

	"github.com/mahm0udnasr/tasker/internal/domain"
	"github.com/mahm0udnasr/tasker/internal/store"
)

// SeedSamples adds a small "Getting started" project whose tasks each
// demonstrate a feature. It is offered on the first-run welcome screen.
func SeedSamples(ctx context.Context, s store.Store, now time.Time) error {
	p, _, err := ResolveProject(ctx, s, "Getting started", now)
	if err != nil {
		return err
	}
	if p.Icon == "" {
		p.Icon = "🚀"
		p.UpdatedAt = now
		if err := s.UpdateProject(ctx, &p); err != nil {
			return err
		}
	}
	today := domain.StartOfDay(now)
	at := func(days, hour int) *time.Time {
		t := today.AddDate(0, 0, days).Add(time.Duration(hour) * time.Hour)
		return &t
	}
	type sample struct {
		title, notes string
		due          *time.Time
		timed        bool
		prio         domain.Priority
		tags         []string
		estimate     int
		subtasks     []string
		inbox        bool
	}
	samples := []sample{
		{title: "Complete me: select and press x", due: at(0, 0), prio: domain.PriorityMedium, tags: []string{"tips"}},
		{title: "Focus on me: press P for a pomodoro", due: at(0, 17), timed: true, estimate: 2,
			tags: []string{"tips", "focus"},
			notes: "A **pomodoro** is 25 minutes of focus, then a short break.\n\n" +
				"- `P` start / pause\n- `z` full-screen focus mode\n- `X` stop"},
		{title: "This one is overdue", due: at(-2, 0), prio: domain.PriorityHigh},
		{title: "Plan the week", due: at(3, 0), prio: domain.PriorityLow,
			subtasks: []string{"Review the calendar", "Pick three priorities", "Block focus time"},
			notes: "Press **enter** on this task to see its subtasks, and **A** to add one.\n\n" +
				"Notes are *markdown*:\n\n- [x] lists\n- [ ] checkboxes\n- `code`"},
		{title: "Find me with / (try typing “tips”)", tags: []string{"tips"}},
		{title: "Captured with no project lands in the Inbox", inbox: true},
	}
	for i, sm := range samples {
		t := domain.NewTask(sm.title, now.Add(time.Duration(i)*time.Millisecond))
		t.Description, t.Priority, t.Estimate = sm.notes, sm.prio, sm.estimate
		t.SetDue(sm.due, sm.timed, now)
		t.Tags = sm.tags
		if !sm.inbox {
			t.ProjectID = p.ID
		}
		if err := s.CreateTask(ctx, &t); err != nil {
			return err
		}
		for _, st := range sm.subtasks {
			sub := domain.NewTask(st, now)
			sub.ParentID = t.ID
			if err := s.CreateTask(ctx, &sub); err != nil {
				return err
			}
		}
	}
	return nil
}
