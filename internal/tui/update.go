package tui

import (
	"errors"

	tea "charm.land/bubbletea/v2"

	"github.com/mahm0udnasr/tasker/internal/tui/common"
	"github.com/mahm0udnasr/tasker/internal/tui/overlay"
	"github.com/mahm0udnasr/tasker/internal/tui/tasklist"
)

// Update implements tea.Model. Routing order:
//  1. data and lifecycle messages are handled here regardless of overlays;
//  2. key presses go to the top overlay if there is one;
//  3. otherwise registry actions (keys bound in actions.go), then pane
//     focus keys, then the focused pane's own navigation.
//
// Anything unrecognized (cursor blinks, huh internals, status timers) is
// forwarded to the top overlay and the status bar.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.layout()
		return m, tea.Batch(m.forwardToOverlay(msg), m.syncDetail(false))

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
		return m, m.syncDetail(true)

	case detailLoadedMsg:
		if msg.gen != m.detailGen {
			return m, nil
		}
		if msg.err != nil {
			return m, common.ErrorCmd(msg.err)
		}
		proj := m.project("")
		if msg.task != nil {
			proj = m.project(msg.task.ProjectID)
		}
		m.detail.SetTask(msg.task, msg.subtasks, proj)
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

	case searchIndexMsg:
		if msg.err != nil {
			return m, common.ErrorCmd(msg.err)
		}
		return m, m.openSearch(msg.tasks)

	case mutatedMsg:
		if msg.undo != nil && !msg.isUndo {
			m.undo.push(*msg.undo)
		}
		if msg.expandID != "" {
			m.list.Expand(msg.expandID)
		}
		// A new task isn't in the list yet; select it once the reload lands.
		m.pendingSelect = msg.selectID
		cmds := []tea.Cmd{m.reload()}
		if msg.text != "" {
			cmds = append(cmds, common.Status(msg.text))
		}
		return m, tea.Batch(cmds...)

	case errMsg:
		return m, tea.Batch(common.ErrorCmd(errors.New(friendly(msg.err))), m.reload())

	case tickMsg:
		return m, tea.Batch(m.reload(), tick())

	case common.ScopeSelectedMsg:
		m.setScope(msg.Scope)
		m.list.ResetCursor()
		return m, m.reloadTasks()

	case jumpScopeMsg:
		m.focus = paneList
		return m, m.jumpScope(msg.scope)

	case jumpToTaskMsg:
		return m, m.jumpToTask(msg)

	case paletteMsg:
		if a, ok := findAction(msg.name); ok {
			return m, m.runAction(a)
		}
		return m, nil

	case overlay.RefineMsg:
		return m, m.setRefinement(msg.Refinement)

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
		return m, m.finishTaskAction(msg)

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
	if a, ok := m.keyAction(msg); ok {
		cmd := m.runAction(a)
		return m, cmd
	}

	k := m.ctx.Keys
	switch {
	case k.Is(msg, common.ActNextPane):
		m.cycleFocus(1)
	case k.Is(msg, common.ActPrevPane):
		m.cycleFocus(-1)
	case k.Is(msg, common.ActLeft):
		if m.focus == paneDetail {
			m.focus = paneList
		} else {
			m.focus = paneSidebar
		}
	case k.Is(msg, common.ActRight):
		if m.focus == paneList && m.detailVisible() {
			m.focus = paneDetail
		} else if m.focus == paneSidebar {
			m.focus = paneList
		}
	default:
		return m.handlePaneKey(msg)
	}
	m.applyFocus()
	m.layout() // narrow terminals show only the focused pane
	return m, nil
}

// handlePaneKey gives navigation keys to the focused pane.
func (m Model) handlePaneKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
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
	case paneDetail:
		m.detail, cmd = m.detail.Update(msg)
		return m, cmd
	default:
		m.list, cmd = m.list.Update(msg)
		return m, tea.Batch(cmd, m.syncDetail(false))
	}
}
