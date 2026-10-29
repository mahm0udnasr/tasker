// Package tui is tasker's full-screen interface. The root Model owns layout,
// focus and the overlay stack, routes messages to small child models, and
// turns their intents into store commands (see cmds.go).
package tui

import (
	"fmt"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/mahm0udnasr/tasker/internal/config"
	"github.com/mahm0udnasr/tasker/internal/domain"
	"github.com/mahm0udnasr/tasker/internal/store"
	"github.com/mahm0udnasr/tasker/internal/theme"
	"github.com/mahm0udnasr/tasker/internal/tui/common"
	"github.com/mahm0udnasr/tasker/internal/tui/sidebar"
	"github.com/mahm0udnasr/tasker/internal/tui/statusbar"
	"github.com/mahm0udnasr/tasker/internal/tui/tasklist"
)

type pane int

const (
	paneSidebar pane = iota
	paneList
	paneCount
)

// refreshEvery reloads periodically so changes from the CLI show up and
// relative dates ("today") roll over at midnight.
const refreshEvery = time.Minute

type tickMsg struct{}

// Model is the root model.
type Model struct {
	ctx      *common.Context
	store    store.Store
	cfg      config.Config
	width    int
	height   int
	focus    pane
	scope    common.Scope
	sidebar  sidebar.Model
	list     tasklist.Model
	status   statusbar.Model
	overlays []common.Overlay
	projects []domain.Project
	counts   domain.Counts
	loadGen  int
	// pendingSelect is a task ID to select when the next list load arrives.
	pendingSelect string
}

// New builds the root model.
func New(s store.Store, cfg config.Config, now func() time.Time) Model {
	ctx := &common.Context{
		Styles: theme.NewStyles(theme.Get(cfg.Theme)),
		Icons:  theme.IconSet(cfg.Icons),
		Keys:   common.NewKeyMap(cfg.Keys),
		Now:    now,
	}
	start := domain.SmartView(cfg.DefaultView)
	if !validView(start) {
		start = domain.ViewToday
	}
	m := Model{
		ctx:     ctx,
		store:   s,
		cfg:     cfg,
		focus:   paneList,
		sidebar: sidebar.New(ctx, start),
		list:    tasklist.New(ctx),
		status:  statusbar.New(ctx),
	}
	m.setScope(m.sidebar.Selected())
	m.applyFocus()
	m.loadGen = 1 // Init has a value receiver, so the first generation is set here
	return m
}

func validView(v domain.SmartView) bool {
	for _, x := range domain.SmartViews {
		if x == v {
			return true
		}
	}
	return false
}

// Run starts the program and blocks until the user quits.
func Run(s store.Store, cfg config.Config) error {
	p := tea.NewProgram(New(s, cfg, time.Now))
	_, err := p.Run()
	return err
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		loadTasks(m.store, m.loadGen, m.scope, m.list.Expanded(), domain.DefaultSort(m.scope.View), m.ctx.Now()),
		loadSidebar(m.store, m.ctx.Now()),
		tick(),
	)
}

func tick() tea.Cmd {
	return tea.Tick(refreshEvery, func(time.Time) tea.Msg { return tickMsg{} })
}

// reload fetches the current list and the sidebar data in parallel.
func (m *Model) reload() tea.Cmd {
	return tea.Batch(m.reloadTasks(), loadSidebar(m.store, m.ctx.Now()))
}

func (m *Model) reloadTasks() tea.Cmd {
	m.loadGen++
	return loadTasks(m.store, m.loadGen, m.scope, m.list.Expanded(), domain.DefaultSort(m.scope.View), m.ctx.Now())
}

func (m *Model) setScope(s common.Scope) {
	m.scope = s
	m.list.SetTitle(s.Title, emptyMessage(s))
	m.status.SetView(s.Title)
}

func emptyMessage(s common.Scope) string {
	switch {
	case s.ProjectID != "":
		return "No open tasks in " + s.Title
	case s.Tag != "":
		return "Nothing tagged " + s.Title
	}
	switch s.View {
	case domain.ViewInbox:
		return "Inbox zero. Nice."
	case domain.ViewToday:
		return "Nothing due today. Enjoy the calm."
	case domain.ViewUpcoming:
		return "Nothing scheduled ahead."
	case domain.ViewOverdue:
		return "Nothing overdue. You're on top of it."
	case domain.ViewCompleted:
		return "No completed tasks yet. You've got this."
	}
	return "No tasks."
}

func (m *Model) applyFocus() {
	overlay := len(m.overlays) > 0
	m.sidebar.SetFocused(m.focus == paneSidebar && !overlay)
	m.list.SetFocused(m.focus == paneList && !overlay)
	m.status.SetHints(m.hints())
}

// hints are the status-bar shortcuts for the focused pane.
func (m *Model) hints() []key.Binding {
	k := m.ctx.Keys
	if m.focus == paneSidebar {
		return k.Hints(common.ActRight, common.ActAdd, common.ActHelp, common.ActQuit)
	}
	return k.Hints(common.ActAdd, common.ActToggleDone, common.ActEdit, common.ActHelp)
}

// projectNames lists project names for form suggestions.
func (m Model) projectNames() []string {
	out := make([]string, len(m.projects))
	for i, p := range m.projects {
		out[i] = p.Name
	}
	return out
}

func (m Model) projectName(id string) string {
	for _, p := range m.projects {
		if p.ID == id {
			return p.Name
		}
	}
	return ""
}

func plural(n int, word string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, word)
	}
	return fmt.Sprintf("%d %ss", n, word)
}
