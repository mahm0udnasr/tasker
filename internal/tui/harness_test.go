package tui

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/mahm0udnasr/tasker/internal/config"
	"github.com/mahm0udnasr/tasker/internal/domain"
	"github.com/mahm0udnasr/tasker/internal/store/sqlite"
)

// harness drives the root model without a terminal: it feeds messages,
// executes the returned commands synchronously, and renders plain text.
type harness struct {
	t     *testing.T
	m     tea.Model
	now   time.Time
	store *sqlite.Store
}

var cmdSliceType = reflect.TypeOf([]tea.Cmd(nil))

func newHarness(t *testing.T, w, h int) *harness {
	t.Helper()
	s, err := sqlite.Open(context.Background(), filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	hs := &harness{t: t, store: s, now: time.Date(2026, 10, 29, 14, 30, 0, 0, time.Local)}
	cfg := config.Default()
	hs.m = New(s, cfg, func() time.Time { return hs.now })
	hs.send(tea.WindowSizeMsg{Width: w, Height: h})
	hs.run(hs.m.Init())
	return hs
}

// run executes cmd and feeds resulting messages back, depth-first. Commands
// that don't return promptly (timers like tea.Tick) are skipped.
func (h *harness) run(cmd tea.Cmd) {
	if cmd == nil {
		return
	}
	done := make(chan tea.Msg, 1)
	go func() { done <- cmd() }()
	var msg tea.Msg
	select {
	case msg = <-done:
	case <-time.After(60 * time.Millisecond):
		return
	}
	switch msg := msg.(type) {
	case nil:
	case tea.BatchMsg:
		for _, c := range msg {
			h.run(c)
		}
	case tea.QuitMsg:
	default:
		// tea.Sequence produces an unexported []Cmd type.
		if v := reflect.ValueOf(msg); v.Kind() == reflect.Slice && v.Type().ConvertibleTo(cmdSliceType) {
			for _, c := range v.Convert(cmdSliceType).Interface().([]tea.Cmd) {
				h.run(c)
			}
			return
		}
		h.send(msg)
	}
}

func (h *harness) send(msg tea.Msg) {
	var cmd tea.Cmd
	h.m, cmd = h.m.Update(msg)
	h.run(cmd)
}

// keys sends key presses; multi-char names ("enter", "ctrl+s") are single keys,
// and a "type:" prefix types text rune by rune.
func (h *harness) keys(keys ...string) {
	for _, k := range keys {
		if text, ok := strings.CutPrefix(k, "type:"); ok {
			for _, r := range text {
				h.send(tea.KeyPressMsg{Code: r, Text: string(r)})
			}
			continue
		}
		h.send(keyMsg(k))
	}
}

func keyMsg(k string) tea.KeyPressMsg {
	named := map[string]rune{
		"enter": tea.KeyEnter, "esc": tea.KeyEscape, "tab": tea.KeyTab, "space": tea.KeySpace,
		"up": tea.KeyUp, "down": tea.KeyDown, "left": tea.KeyLeft, "right": tea.KeyRight,
		"backspace": tea.KeyBackspace,
	}
	if mod, rest, ok := strings.Cut(k, "+"); ok && mod == "ctrl" {
		return tea.KeyPressMsg{Code: rune(rest[0]), Mod: tea.ModCtrl}
	}
	if mod, rest, ok := strings.Cut(k, "+"); ok && mod == "shift" && rest == "tab" {
		return tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift}
	}
	if r, ok := named[k]; ok {
		if r == tea.KeySpace {
			return tea.KeyPressMsg{Code: r, Text: " "}
		}
		return tea.KeyPressMsg{Code: r}
	}
	r := []rune(k)[0]
	return tea.KeyPressMsg{Code: r, Text: k}
}

func (h *harness) screen() string {
	return ansi.Strip(h.m.View().Content)
}

func (h *harness) contains(want string) {
	h.t.Helper()
	if s := h.screen(); !strings.Contains(s, want) {
		h.t.Fatalf("screen does not contain %q:\n%s", want, s)
	}
}

func (h *harness) notContains(want string) {
	h.t.Helper()
	if s := h.screen(); strings.Contains(s, want) {
		h.t.Fatalf("screen unexpectedly contains %q:\n%s", want, s)
	}
}

func (h *harness) seed(title string, mod func(*domain.Task)) domain.Task {
	h.t.Helper()
	t := domain.NewTask(title, h.now)
	if mod != nil {
		mod(&t)
	}
	if err := h.store.CreateTask(context.Background(), &t); err != nil {
		h.t.Fatal(err)
	}
	return t
}

// dump writes the current screen to $TASKER_SNAPSHOT_DIR for eyeballing.
func (h *harness) dump(name string) {
	dir := os.Getenv("TASKER_SNAPSHOT_DIR")
	if dir == "" {
		return
	}
	_ = os.WriteFile(filepath.Join(dir, name+".txt"), []byte(h.screen()), 0o600)
	_ = os.WriteFile(filepath.Join(dir, name+".ansi"), []byte(h.m.View().Content), 0o600)
}

// assertGeometry checks the frame is exactly w×h cells.
func assertGeometry(t *testing.T, h *harness, w, ht int) {
	t.Helper()
	lines := strings.Split(h.screen(), "\n")
	if len(lines) != ht {
		t.Errorf("%dx%d: %d lines", w, ht, len(lines))
	}
	for i, l := range lines {
		if lw := ansi.StringWidth(l); lw != w {
			t.Errorf("%dx%d: line %d is %d wide: %q", w, ht, i, lw, l)
			return
		}
	}
}

func configDefault() config.Config { return config.Default() }

func teaSize(w, h int) tea.WindowSizeMsg { return tea.WindowSizeMsg{Width: w, Height: h} }
