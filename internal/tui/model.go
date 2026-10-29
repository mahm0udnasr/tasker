// Package tui is tasker's full-screen interface. The root Model owns layout,
// focus and the overlay stack, routes messages to small child models, and
// turns their intents into store commands (see cmds.go). User-facing actions
// live in one registry (actions.go) shared by key bindings and the command
// palette.
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
	"github.com/mahm0udnasr/tasker/internal/tui/detail"
	"github.com/mahm0udnasr/tasker/internal/tui/overlay"
	"github.com/mahm0udnasr/tasker/internal/tui/pomodoro"
	"github.com/mahm0udnasr/tasker/internal/tui/sidebar"
	"github.com/mahm0udnasr/tasker/internal/tui/statusbar"
	"github.com/mahm0udnasr/tasker/internal/tui/tasklist"
)

type pane int

const (
	paneSidebar pane = iota
	paneList
	paneDetail
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
	cfgPath  string // "" disables saving (tests)
	width    int
	height   int
	focus    pane
	scope    common.Scope
	sidebar  sidebar.Model
	list     tasklist.Model
	detail   detail.Model
	status   statusbar.Model
	overlays []common.Overlay
	projects []domain.Project
	tags     []domain.Tag
	counts   domain.Counts
	undo     undoStack
	// pomo is shared by pointer so the status bar, list and focus overlay
	// render one live timer (Update runs on a single goroutine).
	pomo *pomodoro.Model

	showDetail bool                      // user preference; hidden anyway when too narrow
	sorts      map[string]domain.SortKey // per-scope sort overrides
	refine     domain.Refinement

	// themeBefore is the theme to restore if the theme picker is cancelled.
	themeBefore string

	loadGen   int
	detailGen int
	// pendingSelect is a task ID to select when the next list load arrives.
	pendingSelect string
}

// New builds the root model. cfgPath is where preferences (theme, welcome
// seen) are saved; pass "" to never write the config.
func New(s store.Store, cfg config.Config, now func() time.Time) Model {
	return NewWithConfigPath(s, cfg, "", now)
}

// NewWithConfigPath is New with a config file to save preferences to.
func NewWithConfigPath(s store.Store, cfg config.Config, cfgPath string, now func() time.Time) Model {
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
		ctx:        ctx,
		store:      s,
		cfg:        cfg,
		cfgPath:    cfgPath,
		focus:      paneList,
		sidebar:    sidebar.New(ctx, start),
		list:       tasklist.New(ctx),
		detail:     detail.New(ctx),
		status:     statusbar.New(ctx),
		showDetail: cfg.ShowDetail,
		sorts:      map[string]domain.SortKey{},
		pomo: pomodoro.New(ctx, s, pomodoro.Options{
			Settings: cfg.Pomodoro.Settings(), Notify: cfg.Pomodoro.Notify, Bell: cfg.Pomodoro.Bell,
		}),
	}
	m.setScope(m.sidebar.Selected())
	m.applyFocus()
	m.loadGen = 1 // Init has a value receiver, so the first generation is set here
	if !cfg.Seen && cfgPath != "" {
		m.overlays = append(m.overlays, overlay.NewWelcome(ctx))
		m.applyFocus()
	}
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
func Run(s store.Store, cfg config.Config, cfgPath string) error {
	p := tea.NewProgram(NewWithConfigPath(s, cfg, cfgPath, time.Now))
	_, err := p.Run()
	return err
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		loadTasks(m.store, m.loadGen, m.query(), m.ctx.Now()),
		loadSidebar(m.store, m.ctx.Now()),
		m.pomo.Init(),
		tick(),
	)
}

func tick() tea.Cmd {
	return tea.Tick(refreshEvery, func(time.Time) tea.Msg { return tickMsg{} })
}

func (m *Model) query() listQuery {
	return listQuery{scope: m.scope, expanded: m.list.Expanded(), sort: m.sortKey(), refine: m.refine}
}

// sortKey is the user's choice for this scope, or the view's natural order.
func (m *Model) sortKey() domain.SortKey {
	if k, ok := m.sorts[m.scope.Key()]; ok {
		return k
	}
	return domain.DefaultSort(m.scope.View)
}

// reload fetches the current list and the sidebar data in parallel.
func (m *Model) reload() tea.Cmd {
	return tea.Batch(m.reloadTasks(), loadSidebar(m.store, m.ctx.Now()))
}

func (m *Model) reloadTasks() tea.Cmd {
	m.loadGen++
	return loadTasks(m.store, m.loadGen, m.query(), m.ctx.Now())
}

// syncDetail loads the selected task into the detail pane when the
// selection changed (or always, with force, after data changed).
func (m *Model) syncDetail(force bool) tea.Cmd {
	id := ""
	if t, ok := m.list.Selected(); ok {
		id = t.ID
	}
	if id == m.detail.TaskID() && !force {
		return nil
	}
	m.detailGen++
	return loadDetail(m.store, m.detailGen, id)
}

func (m *Model) setScope(s common.Scope) {
	m.scope = s
	m.updateListTitle()
	m.status.SetView(s.Title)
}

// updateListTitle shows the scope plus any non-default sort and filter.
func (m *Model) updateListTitle() {
	var badges []string
	if k := m.sortKey(); k != domain.DefaultSort(m.scope.View) {
		badges = append(badges, "↕ "+string(k))
	}
	if m.refine.Active() {
		badges = append(badges, "⧩ "+m.refine.Describe())
	}
	m.list.SetTitle(m.scope.Title, emptyMessage(m.scope, m.refine.Active()))
	m.list.SetBadges(badges)
}

func emptyMessage(s common.Scope, filtered bool) string {
	if filtered {
		return "Nothing matches the filter. Press F to clear it."
	}
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

// panes lists the focusable panes in Tab order.
func (m *Model) panes() []pane {
	if m.detailVisible() {
		return []pane{paneSidebar, paneList, paneDetail}
	}
	return []pane{paneSidebar, paneList}
}

func (m *Model) cycleFocus(delta int) {
	ps := m.panes()
	i := 0
	for j, p := range ps {
		if p == m.focus {
			i = j
		}
	}
	m.focus = ps[(i+delta+len(ps))%len(ps)]
}

func (m *Model) applyFocus() {
	overlay := len(m.overlays) > 0
	if m.focus == paneDetail && !m.detailVisible() {
		m.focus = paneList
	}
	m.sidebar.SetFocused(m.focus == paneSidebar && !overlay)
	m.list.SetFocused(m.focus == paneList && !overlay)
	m.detail.SetFocused(m.focus == paneDetail && !overlay)
	m.status.SetHints(m.hints())
}

// hints are the status-bar shortcuts for the focused pane.
func (m *Model) hints() []key.Binding {
	k := m.ctx.Keys
	switch m.focus {
	case paneSidebar:
		return k.Hints(common.ActRight, common.ActAdd, common.ActPalette, common.ActHelp)
	case paneDetail:
		return k.Hints(common.ActEdit, common.ActDetail, common.ActHelp)
	}
	return k.Hints(common.ActAdd, common.ActToggleDone, common.ActSearch, common.ActPalette, common.ActHelp)
}

func (m Model) projectNames() []string {
	out := make([]string, len(m.projects))
	for i, p := range m.projects {
		out[i] = p.Name
	}
	return out
}

func (m Model) project(id string) domain.Project {
	for _, p := range m.projects {
		if p.ID == id {
			return p
		}
	}
	return domain.Project{}
}

func (m Model) projectName(id string) string { return m.project(id).Name }

func plural(n int, word string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, word)
	}
	return fmt.Sprintf("%d %ss", n, word)
}
