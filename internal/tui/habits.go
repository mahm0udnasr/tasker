package tui

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/mahm0udnasr/tasker/internal/domain"
	"github.com/mahm0udnasr/tasker/internal/store"
	"github.com/mahm0udnasr/tasker/internal/tui/common"
	"github.com/mahm0udnasr/tasker/internal/tui/habitlist"
	"github.com/mahm0udnasr/tasker/internal/tui/overlay"
)

// The habits view reuses the task keys for the equivalent actions: add (a),
// rename (e), toggle today (x/space) and delete (d). Task-only actions are
// refused there rather than acting on a task the user can't see.

var errNoHabit = errors.New("no habit selected")

// habitAction runs a in the habits view. handled is false for actions that
// work the same in every view (search, palette, theme, quit, …).
func (m *Model) habitAction(a action) (cmd tea.Cmd, handled bool) {
	switch a.name {
	case common.ActAdd:
		return m.openHabitPrompt(nil), true
	case common.ActEdit, common.ActToggleDone, common.ActDelete:
		r, ok := m.habits.Selected()
		if !ok {
			return common.ErrorCmd(errNoHabit), true
		}
		switch a.name {
		case common.ActEdit:
			return m.openHabitPrompt(&r.Habit), true
		case common.ActToggleDone:
			return m.toggleHabit(r), true
		default:
			return m.confirmDeleteHabit(r.Habit), true
		}
	}
	if !availableInHabits(a) {
		return common.ErrorCmd(fmt.Errorf("%s isn't available in Habits", a.title)), true
	}
	return nil, false
}

// availableInHabits reports whether a makes sense in the habits view.
func availableInHabits(a action) bool {
	if a.needsTask {
		return false
	}
	switch a.name {
	case common.ActFilter, common.ActClearFilter, common.ActSort, common.ActDetail, common.ActPomodoro:
		return false
	}
	return true
}

// habitTitles relabels shared actions in the palette while in Habits.
var habitTitles = map[string]string{
	common.ActAdd:        "Add habit",
	common.ActEdit:       "Rename habit",
	common.ActToggleDone: "Toggle today",
	common.ActDelete:     "Delete habit",
}

func (m *Model) habitHints() []key.Binding {
	k := m.ctx.Keys
	out := make([]key.Binding, 0, len(common.HabitActions)+1)
	for _, a := range common.HabitActions {
		out = append(out, k.Habit(a))
	}
	return append(out, k.Get(common.ActHelp))
}

type habitActionKind int

const (
	habitAdd habitActionKind = iota
	habitRename
	habitDelete
)

// habitActionMsg is produced by the prompt or confirm dialog once the user
// has supplied what a habit action needs.
type habitActionMsg struct {
	kind  habitActionKind
	habit domain.Habit
	value string
}

func (m *Model) finishHabitAction(a habitActionMsg) tea.Cmd {
	now := m.ctx.Now()
	switch a.kind {
	case habitAdd:
		return createHabit(m.store, a.value, now)
	case habitRename:
		return renameHabit(m.store, a.habit.ID, a.value, now)
	case habitDelete:
		return deleteHabit(m.store, a.habit, now)
	}
	return nil
}

// openHabitPrompt asks for a name: a new habit when h is nil, else a rename.
func (m *Model) openHabitPrompt(h *domain.Habit) tea.Cmd {
	o := overlay.PromptOptions{
		Title:       "New habit",
		Placeholder: `"Read 20 pages", "Stretch", "No sugar"`,
		Help:        "enter save · esc cancel",
		Preview: func(s string) (string, bool) {
			probe := domain.Habit{Name: s}
			if err := probe.Validate(); err != nil {
				return m.ctx.Icons.Cross + " give it a name", false
			}
			return "→ " + m.ctx.Icons.Habits + " daily", true
		},
		OnSubmit: func(s string) tea.Msg { return habitActionMsg{kind: habitAdd, value: s} },
	}
	if h != nil {
		habit := *h
		o.Title, o.Value = "Rename · "+habit.Name, habit.Name
		o.OnSubmit = func(s string) tea.Msg { return habitActionMsg{kind: habitRename, habit: habit, value: s} }
	}
	p, init := overlay.NewPrompt(m.ctx, o)
	return m.pushOverlay(p, init)
}

func (m *Model) confirmDeleteHabit(h domain.Habit) tea.Cmd {
	body := fmt.Sprintf("“%s” and its history will be deleted. You can undo with %s.",
		h.Name, m.ctx.Keys.Get(common.ActUndo).Help().Key)
	return m.pushOverlay(overlay.NewConfirm(m.ctx, "Delete habit?", body,
		habitActionMsg{kind: habitDelete, habit: h}), nil)
}

func (m *Model) toggleHabit(r habitlist.Row) tea.Cmd {
	now := m.ctx.Now()
	day := domain.DayKey(now)
	done := !r.Done[day]
	text := "“" + r.Habit.Name + "” not done today"
	if done {
		after := maps.Clone(r.Done)
		if after == nil {
			after = map[string]bool{}
		}
		after[day] = true
		text = fmt.Sprintf("“%s” done today · %d-day streak", r.Habit.Name, domain.HabitStreak(after, now))
	}
	return setHabitDay(m.store, r.Habit.ID, day, done, text, now)
}

// --- loads & mutations ----------------------------------------------------------

type habitsLoadedMsg struct {
	gen  int
	rows []habitlist.Row
	err  error
}

func (m *Model) reloadHabits() tea.Cmd {
	m.habitGen++
	return loadHabits(m.store, m.habitGen)
}

func loadHabits(s store.Store, gen int) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := dbCtx()
		defer cancel()
		hs, err := s.ListHabits(ctx)
		if err != nil {
			return habitsLoadedMsg{gen: gen, err: err}
		}
		logs, err := s.ListHabitLogs(ctx)
		if err != nil {
			return habitsLoadedMsg{gen: gen, err: err}
		}
		done := make(map[string]map[string]bool, len(hs))
		for _, l := range logs {
			if done[l.HabitID] == nil {
				done[l.HabitID] = map[string]bool{}
			}
			done[l.HabitID][l.Day] = true
		}
		rows := make([]habitlist.Row, len(hs))
		for i, h := range hs {
			rows[i] = habitlist.Row{Habit: h, Done: done[h.ID]}
		}
		return habitsLoadedMsg{gen: gen, rows: rows}
	}
}

func createHabit(s store.Store, name string, now time.Time) tea.Cmd {
	return mutate(func(ctx context.Context) (mutatedMsg, error) {
		h := domain.NewHabit(name, now)
		if err := s.CreateHabit(ctx, &h); err != nil {
			return mutatedMsg{}, err
		}
		text := "Added habit “" + h.Name + "”"
		return mutatedMsg{text: text, selectHabitID: h.ID,
			undo: &undoEntry{desc: text, createdHabitIDs: []string{h.ID}}}, nil
	})
}

func renameHabit(s store.Store, id, name string, now time.Time) tea.Cmd {
	return mutate(func(ctx context.Context) (mutatedMsg, error) {
		h, err := s.GetHabit(ctx, id)
		if err != nil {
			return mutatedMsg{}, err
		}
		before := h
		h.Name, h.UpdatedAt = name, now
		if err := s.UpdateHabit(ctx, &h); err != nil {
			return mutatedMsg{}, err
		}
		text := "Renamed “" + before.Name + "” to “" + h.Name + "”"
		return mutatedMsg{text: text, selectHabitID: h.ID,
			undo: &undoEntry{desc: text, habits: []domain.Habit{before}}}, nil
	})
}

func deleteHabit(s store.Store, h domain.Habit, now time.Time) tea.Cmd {
	return mutate(func(ctx context.Context) (mutatedMsg, error) {
		before, err := s.GetHabit(ctx, h.ID)
		if err != nil {
			return mutatedMsg{}, err
		}
		if err := s.DeleteHabit(ctx, h.ID, now); err != nil {
			return mutatedMsg{}, err
		}
		text := "Deleted habit “" + before.Name + "”"
		return mutatedMsg{text: text + " · u to undo",
			undo: &undoEntry{desc: text, habits: []domain.Habit{before}}}, nil
	})
}

// setHabitDay marks a habit done (or not) on day, undoably.
func setHabitDay(s store.Store, id, day string, done bool, text string, now time.Time) tea.Cmd {
	return mutate(func(ctx context.Context) (mutatedMsg, error) {
		if err := s.SetHabitDone(ctx, id, day, done, now); err != nil {
			return mutatedMsg{}, err
		}
		return mutatedMsg{text: text, selectHabitID: id,
			undo: &undoEntry{desc: text, habitDay: &habitDayState{habitID: id, day: day, done: !done}}}, nil
	})
}

// undoHabits reverses the habit part of an undo entry.
func undoHabits(ctx context.Context, s store.Store, e undoEntry, now time.Time) (selectID string, err error) {
	for _, id := range e.createdHabitIDs {
		if err := s.DeleteHabit(ctx, id, now); err != nil && !errors.Is(err, store.ErrNotFound) {
			return "", err
		}
	}
	habits := make([]domain.Habit, len(e.habits))
	for i, h := range e.habits {
		h.UpdatedAt = now
		habits[i] = h
		selectID = h.ID
	}
	if err := s.RestoreHabits(ctx, habits); err != nil {
		return "", err
	}
	if d := e.habitDay; d != nil {
		if err := s.SetHabitDone(ctx, d.habitID, d.day, d.done, now); err != nil {
			return "", err
		}
		selectID = d.habitID
	}
	return selectID, nil
}
