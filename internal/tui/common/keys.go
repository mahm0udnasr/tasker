package common

import (
	"charm.land/bubbles/v2/key"
)

// Action names are the keys of the [keys] table in config.toml.
const (
	ActUp          = "up"
	ActDown        = "down"
	ActTop         = "top"
	ActBottom      = "bottom"
	ActPageUp      = "page_up"
	ActPageDown    = "page_down"
	ActLeft        = "left"
	ActRight       = "right"
	ActNextPane    = "next_pane"
	ActPrevPane    = "prev_pane"
	ActAdd         = "add"
	ActAddSubtask  = "add_subtask"
	ActEdit        = "edit"
	ActToggleDone  = "toggle_done"
	ActToggleStart = "toggle_progress"
	ActDelete      = "delete"
	ActPriority    = "priority"
	ActDue         = "due"
	ActMove        = "move"
	ActExpand      = "expand"
	ActHelp        = "help"
	ActQuit        = "quit"
	ActRefresh     = "refresh"
	ActDelProject  = "delete_project"
	ActMoveUp      = "move_up"
	ActMoveDown    = "move_down"
	ActSearch      = "search"
	ActFilter      = "filter"
	ActClearFilter = "clear_filter"
	ActSort        = "sort"
	ActUndo        = "undo"
	ActDetail      = "toggle_detail"
	ActPalette     = "palette"
	ActPomodoro    = "pomodoro"
	ActPomoStop    = "pomodoro_stop"
	ActPomoSkip    = "pomodoro_skip"
	ActFocusMode   = "focus_mode"
	ActStats       = "stats"
	ActTheme       = "theme"
)

type actionDef struct {
	name string
	keys []string
	help string // key label shown in help; "" = derive from keys
	desc string
}

// defaults lists every action in help order.
var defaults = []actionDef{
	{ActUp, []string{"k", "up"}, "k/↑", "move up"},
	{ActDown, []string{"j", "down"}, "j/↓", "move down"},
	{ActTop, []string{"g", "home"}, "g", "go to top"},
	{ActBottom, []string{"G", "end"}, "G", "go to bottom"},
	{ActPageUp, []string{"ctrl+u", "pgup"}, "ctrl+u", "page up"},
	{ActPageDown, []string{"ctrl+d", "pgdown"}, "ctrl+d", "page down"},
	{ActLeft, []string{"h", "left"}, "h/←", "focus left pane"},
	{ActRight, []string{"l", "right"}, "l/→", "focus right pane"},
	{ActNextPane, []string{"tab"}, "tab", "next pane"},
	{ActPrevPane, []string{"shift+tab"}, "shift+tab", "previous pane"},
	{ActAdd, []string{"a"}, "", "add task"},
	{ActAddSubtask, []string{"A"}, "", "add subtask"},
	{ActEdit, []string{"e"}, "", "edit task"},
	{ActToggleDone, []string{"x", "space"}, "x/space", "toggle done"},
	{ActToggleStart, []string{"i"}, "", "toggle in progress"},
	{ActDelete, []string{"d"}, "", "delete task"},
	{ActPriority, []string{"p"}, "", "cycle priority"},
	{ActDue, []string{"t"}, "", "set due date"},
	{ActMove, []string{"m"}, "", "move to project"},
	{ActExpand, []string{"enter", "o"}, "enter", "expand subtasks"},
	{ActMoveUp, []string{"K"}, "", "move task up (manual sort)"},
	{ActMoveDown, []string{"J"}, "", "move task down (manual sort)"},
	{ActDelProject, []string{"D"}, "", "delete selected project"},
	{ActSearch, []string{"/"}, "", "search all tasks"},
	{ActFilter, []string{"f"}, "", "filter list"},
	{ActClearFilter, []string{"F"}, "", "clear filter"},
	{ActSort, []string{"s"}, "", "cycle sort"},
	{ActUndo, []string{"u"}, "", "undo"},
	{ActDetail, []string{"v"}, "", "toggle detail pane"},
	{ActPalette, []string{"ctrl+p", ":"}, "ctrl+p", "command palette"},
	{ActPomodoro, []string{"P"}, "", "start / pause pomodoro"},
	{ActPomoStop, []string{"X"}, "", "stop pomodoro"},
	{ActPomoSkip, []string{">"}, "", "skip to next phase"},
	{ActFocusMode, []string{"z"}, "", "focus mode"},
	{ActStats, []string{"S"}, "", "stats"},
	{ActTheme, []string{"T"}, "", "change theme"},
	{ActRefresh, []string{"ctrl+r"}, "", "reload"},
	{ActHelp, []string{"?"}, "", "toggle help"},
	{ActQuit, []string{"q", "ctrl+c"}, "q", "quit"},
}

// KeyMap holds a binding per action.
type KeyMap struct {
	bindings map[string]key.Binding
	order    []string
}

// NewKeyMap builds the keymap from defaults plus user overrides
// (action → keys). Unknown actions in overrides are ignored; an override
// replaces all default keys for that action.
func NewKeyMap(overrides map[string][]string) *KeyMap {
	km := &KeyMap{bindings: map[string]key.Binding{}}
	for _, d := range defaults {
		keys, label := d.keys, d.help
		if o, ok := overrides[d.name]; ok && len(o) > 0 {
			keys, label = o, ""
		}
		if label == "" {
			label = keys[0]
			if label == " " {
				label = "space"
			}
		}
		km.bindings[d.name] = key.NewBinding(key.WithKeys(keys...), key.WithHelp(label, d.desc))
		km.order = append(km.order, d.name)
	}
	// ctrl+c always quits, even if the user rebinds quit.
	q := km.bindings[ActQuit]
	if !hasKey(q.Keys(), "ctrl+c") {
		km.bindings[ActQuit] = key.NewBinding(key.WithKeys(append(q.Keys(), "ctrl+c")...), key.WithHelp(q.Help().Key, q.Help().Desc))
	}
	return km
}

func hasKey(keys []string, k string) bool {
	for _, x := range keys {
		if x == k {
			return true
		}
	}
	return false
}

// Get returns the binding for an action.
func (k *KeyMap) Get(action string) key.Binding { return k.bindings[action] }

// Is reports whether msg matches the action.
func (k *KeyMap) Is(msg interface{ String() string }, action string) bool {
	return key.Matches(msg, k.bindings[action])
}

// Ordered returns all bindings in help order.
func (k *KeyMap) Ordered() []key.Binding {
	out := make([]key.Binding, 0, len(k.order))
	for _, a := range k.order {
		out = append(out, k.bindings[a])
	}
	return out
}

// Hints returns a short list of bindings for the status bar.
func (k *KeyMap) Hints(actions ...string) []key.Binding {
	out := make([]key.Binding, 0, len(actions))
	for _, a := range actions {
		out = append(out, k.bindings[a])
	}
	return out
}
