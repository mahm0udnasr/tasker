package tui

import (
	"errors"

	tea "charm.land/bubbletea/v2"

	"github.com/mahm0udnasr/tasker/internal/tui/common"
	"github.com/mahm0udnasr/tasker/internal/tui/tasklist"
)

// Update implements tea.Model. Routing order:
//  1. data and lifecycle messages are handled here regardless of overlays;
//  2. key presses go to the top overlay if there is one;
//  3. otherwise global keys, then the focused pane, then task actions.
//
// Anything unrecognized (cursor blinks, huh internals, status timers) is
// forwarded to the top overlay and the status bar.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.layout()
		return m, m.forwardToOverlay(msg)

	case tasksLoadedMsg:
		if msg.gen != m.loadGen {
			return m, nil // stale: a newer load is in flight
		}
		if msg.err != nil {
			return m, common.ErrorCmd(msg.err)
		}
		m.list.SetRows(msg.rows)
		if m.pendingSelect != "" {
			m.list.SelectID(m.pendingSelect)
			m.pendingSelect = ""
		}
		return m, nil

	case sidebarLoadedMsg:
		if msg.err != nil {
			return m, common.ErrorCmd(msg.err)
		}
		m.projects, m.counts = msg.projects, msg.counts
		m.sidebar.SetData(msg.projects, msg.tags, msg.counts)
		m.status.SetCounts(msg.counts)
		// A deleted project or vanished tag can invalidate the scope.
		if sel := m.sidebar.Selected(); sel.Key() != m.scope.Key() {
			m.setScope(sel)
			m.list.ResetCursor()
			return m, m.reloadTasks()
		}
		return m, nil

	case mutatedMsg:
		if msg.expandID != "" {
			m.list.Expand(msg.expandID)
		}
		// A new task isn't in the list yet; select it once the reload lands.
		m.pendingSelect = msg.selectID
		return m, tea.Batch(m.reload(), common.Status(msg.text))

	case errMsg:
		return m, tea.Batch(common.ErrorCmd(errors.New(friendly(msg.err))), m.reload())

	case tickMsg:
		return m, tea.Batch(m.reload(), tick())

	case common.ScopeSelectedMsg:
		m.setScope(msg.Scope)
		m.list.ResetCursor()
		return m, m.reloadTasks()

	case tasklist.ExpandToggledMsg:
		return m, m.reloadTasks()

	case common.CloseOverlayMsg:
		if len(m.overlays) > 0 {
			m.overlays = m.overlays[:len(m.overlays)-1]
		}
		m.applyFocus()
		return m, nil

	case common.SaveDraftMsg:
		return m, saveDraft(m.store, msg.Draft, m.ctx.Now())

	case common.StatusMsg:
		var cmd tea.Cmd
		m.status, cmd = m.status.Update(msg)
		return m, cmd

	case taskActionMsg:
		return m, m.runAction(msg)

	case deleteProjectMsg:
		return m, deleteProject(m.store, msg.project, m.ctx.Now())

	case tea.KeyPressMsg:
		if len(m.overlays) > 0 {
			return m, m.forwardToOverlay(msg)
		}
		return m.handleKey(msg)
	}

	var cmds []tea.Cmd
	cmds = append(cmds, m.forwardToOverlay(msg))
	var cmd tea.Cmd
	m.status, cmd = m.status.Update(msg)
	return m, tea.Batch(append(cmds, cmd)...)
}

func (m *Model) forwardToOverlay(msg tea.Msg) tea.Cmd {
	if len(m.overlays) == 0 {
		return nil
	}
	top := len(m.overlays) - 1
	var cmd tea.Cmd
	m.overlays[top], cmd = m.overlays[top].Update(msg)
	return cmd
}

// pushOverlay opens a modal; init is the overlay's start-up command.
func (m *Model) pushOverlay(o common.Overlay, init tea.Cmd) tea.Cmd {
	m.overlays = append(m.overlays, o)
	m.applyFocus()
	return init
}

func (m Model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	k := m.ctx.Keys
	switch {
	case k.Is(msg, common.ActQuit):
		return m, tea.Quit
	case k.Is(msg, common.ActHelp):
		return m, m.openHelp()
	case k.Is(msg, common.ActRefresh):
		return m, tea.Batch(m.reload(), common.Status("Reloaded"))
	case k.Is(msg, common.ActNextPane):
		m.focus = (m.focus + 1) % paneCount
	case k.Is(msg, common.ActPrevPane):
		m.focus = (m.focus + paneCount - 1) % paneCount
	case k.Is(msg, common.ActLeft):
		m.focus = paneSidebar
	case k.Is(msg, common.ActRight):
		m.focus = paneList
	default:
		return m.handlePaneKey(msg)
	}
	m.applyFocus()
	m.layout() // narrow terminals show only the focused pane
	return m, nil
}

func (m Model) handlePaneKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	// These work from either pane and target the sidebar selection.
	switch {
	case m.ctx.Keys.Is(msg, common.ActAdd):
		return m, m.openAddForm("", "")
	case m.ctx.Keys.Is(msg, common.ActDelProject):
		return m, m.confirmDeleteProject()
	}
	var cmd tea.Cmd
	switch m.focus {
	case paneSidebar:
		m.sidebar, cmd = m.sidebar.Update(msg)
		if cmd == nil && m.ctx.Keys.Is(msg, common.ActExpand) {
			m.focus = paneList // enter on a view jumps into its list
			m.applyFocus()
			m.layout()
		}
		return m, cmd
	default:
		if c := m.taskKey(msg); c != nil {
			return m, c
		}
		m.list, cmd = m.list.Update(msg)
		return m, cmd
	}
}
