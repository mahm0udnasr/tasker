// Package common holds what every TUI component shares: the render context
// (styles, icons, keys, clock), the selection Scope, and the messages that
// components use to talk to the root model. Components never call the
// store; they emit messages and the root turns them into commands.
package common

import (
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/mahm0udnasr/tasker/internal/domain"
	"github.com/mahm0udnasr/tasker/internal/theme"
)

// Tick schedules a timer message. All of tasker's own timers go through it
// so tests can replace it with a no-op and drive time explicitly, instead of
// guessing which commands are timers by how long they take.
var Tick = tea.Tick

// Context is shared by pointer, so a theme switch reaches every component on
// the next render.
type Context struct {
	Styles *theme.Styles
	Icons  theme.Icons
	Keys   *KeyMap
	Now    func() time.Time
	// Width and Height are the terminal size, kept current by the root.
	Width, Height int
}

// Scope is what the sidebar selected: a smart view, a project or a tag.
type Scope struct {
	View      domain.SmartView
	ProjectID string
	Tag       string
	Title     string
}

// Filter turns the scope into a store filter.
func (s Scope) Filter(now time.Time) domain.Filter {
	f := domain.Filter{View: s.View, ProjectID: s.ProjectID, Tag: s.Tag, Now: now}
	if f.View == "" {
		f.View = domain.ViewAll
	}
	return f
}

// Key identifies the scope for equality checks.
func (s Scope) Key() string { return string(s.View) + "|" + s.ProjectID + "|" + s.Tag }

// ScopeSelectedMsg is sent by the sidebar when its selection changes.
type ScopeSelectedMsg struct{ Scope Scope }

// StatusMsg shows a transient message in the status bar.
type StatusMsg struct {
	Text  string
	Error bool
}

// Status returns a command that shows an info message.
func Status(text string) tea.Cmd {
	return func() tea.Msg { return StatusMsg{Text: text} }
}

// ErrorCmd returns a command that shows err in the status bar.
func ErrorCmd(err error) tea.Cmd {
	return func() tea.Msg { return StatusMsg{Text: err.Error(), Error: true} }
}

// CloseOverlayMsg pops the top overlay.
type CloseOverlayMsg struct{}

// CloseOverlay is a command that pops the top overlay.
func CloseOverlay() tea.Msg { return CloseOverlayMsg{} }

// TaskDraft is what the add/edit form produces. Fields are raw user input;
// the root resolves them (dates, projects) when saving.
type TaskDraft struct {
	ID          string // empty for a new task
	ParentID    string
	Title       string
	Description string
	Due         string
	Priority    domain.Priority
	Project     string // project name; "" = Inbox
	Tags        string // comma-separated
	Estimate    string // planned pomodoros; "" or "0" for none
}

// SaveDraftMsg asks the root to persist a draft.
type SaveDraftMsg struct{ Draft TaskDraft }

// ConfirmedMsg carries the action a confirm dialog approved.
type ConfirmedMsg struct{ Action tea.Msg }

// Overlay is a modal drawn centered above the panes. The top overlay
// receives every message until it emits CloseOverlayMsg.
type Overlay interface {
	Update(tea.Msg) (Overlay, tea.Cmd)
	View() string
}
