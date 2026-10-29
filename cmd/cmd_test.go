package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// cli runs commands against an isolated database and config.
type cli struct {
	t   *testing.T
	db  string
	now time.Time
}

func newCLI(t *testing.T) *cli {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "cfg"))
	t.Setenv("TASKER_DB", "")
	t.Setenv("NO_COLOR", "1")
	return &cli{t: t, db: filepath.Join(dir, "tasker.db"),
		now: time.Date(2026, 10, 29, 14, 30, 0, 0, time.Local)}
}

func (c *cli) run(args ...string) (string, error) {
	c.t.Helper()
	var out, errOut bytes.Buffer
	root := NewRootCmd(&out, &errOut)
	root.SetArgs(append([]string{"--db", c.db}, args...))
	nowFn = func() time.Time { return c.now } // pin the clock
	defer func() { nowFn = time.Now }()
	err := root.ExecuteContext(context.Background())
	return out.String() + errOut.String(), err
}

func (c *cli) mustRun(args ...string) string {
	c.t.Helper()
	out, err := c.run(args...)
	if err != nil {
		c.t.Fatalf("tasker %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return out
}

func (c *cli) listJSON(args ...string) []jsonTask {
	c.t.Helper()
	var ts []jsonTask
	out := c.mustRun(append([]string{"list", "--json"}, args...)...)
	if err := json.Unmarshal([]byte(out), &ts); err != nil {
		c.t.Fatalf("bad JSON: %v\n%s", err, out)
	}
	return ts
}

func TestAddAndList(t *testing.T) {
	c := newCLI(t)
	out := c.mustRun("add", "Finish ML assignment", "--due", "tomorrow 8pm", "--priority", "high",
		"--project", "uni", "--tag", "ml")
	if !strings.Contains(out, "created project uni") || !strings.Contains(out, "tomorrow 20:00") {
		t.Errorf("add output:\n%s", out)
	}
	c.mustRun("add", "Buy", "milk", "-d", "today")
	c.mustRun("add", "Old report", "-d", "2026-10-20", "-p", "urgent")

	tests := []struct {
		args []string
		want []string
	}{
		{nil, []string{"Finish ML assignment", "Buy milk", "Old report"}},
		{[]string{"--today"}, []string{"Old report", "Buy milk"}}, // Today includes overdue, sorted by due
		{[]string{"--overdue"}, []string{"Old report"}},
		{[]string{"--upcoming"}, []string{"Finish ML assignment"}},
		{[]string{"--project", "UNI"}, []string{"Finish ML assignment"}},
		{[]string{"--tag", "ml"}, []string{"Finish ML assignment"}},
		{[]string{"--sort", "priority"}, []string{"Old report", "Finish ML assignment", "Buy milk"}},
	}
	for _, tt := range tests {
		t.Run(strings.Join(tt.args, " "), func(t *testing.T) {
			var got []string
			for _, task := range c.listJSON(tt.args...) {
				got = append(got, task.Title)
			}
			if strings.Join(got, "|") != strings.Join(tt.want, "|") {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}

	ts := c.listJSON("--project", "uni")
	if ts[0].Project != "uni" || ts[0].Priority.String() != "high" || ts[0].Tags[0] != "ml" {
		t.Errorf("json fields: %+v", ts[0])
	}
}

func TestDoneReopenRm(t *testing.T) {
	c := newCLI(t)
	c.mustRun("add", "one")
	c.mustRun("add", "two")
	ts := c.listJSON()
	one, two := ts[0].ShortID, ts[1].ShortID

	c.mustRun("done", one)
	if got := c.listJSON(); len(got) != 1 || got[0].Title != "two" {
		t.Errorf("after done: %+v", got)
	}
	if got := c.listJSON("--completed"); len(got) != 1 || got[0].CompletedAt == nil {
		t.Errorf("completed view: %+v", got)
	}
	c.mustRun("reopen", one)
	c.mustRun("rm", two)
	if got := c.listJSON(); len(got) != 1 || got[0].Title != "one" {
		t.Errorf("after reopen+rm: %+v", got)
	}
	if _, err := c.run("done", two); err == nil {
		t.Error("done on deleted task should fail")
	}
}

func TestSubtasks(t *testing.T) {
	c := newCLI(t)
	c.mustRun("add", "parent", "-P", "home")
	pid := c.listJSON()[0].ShortID
	c.mustRun("add", "child a", "--parent", pid)
	c.mustRun("add", "child b", "--parent", pid)

	ts := c.listJSON()
	if len(ts) != 1 || len(ts[0].Subtasks) != 2 || ts[0].SubtasksTotal != 2 {
		t.Fatalf("tree: %+v", ts)
	}
	if ts[0].Subtasks[0].Project != "home" {
		t.Errorf("subtask should inherit project: %+v", ts[0].Subtasks[0])
	}
	out := c.mustRun("list", "-s")
	if !strings.Contains(out, "└─ child b") || !strings.Contains(out, "[0/2]") {
		t.Errorf("tree output:\n%s", out)
	}
}

func TestEditFlags(t *testing.T) {
	c := newCLI(t)
	c.mustRun("add", "draft", "-t", "a,b")
	id := c.listJSON()[0].ShortID
	c.mustRun("edit", id, "--title", "final", "--add-tag", "c", "--rm-tag", "a", "--due", "none",
		"--status", "doing", "--project", "work")
	got := c.listJSON()[0]
	if got.Title != "final" || strings.Join(got.Tags, ",") != "b,c" || got.Status != "in_progress" ||
		got.Project != "work" || got.Due != nil {
		t.Errorf("edited: %+v", got)
	}
	c.mustRun("edit", id, "--project", "")
	if got := c.listJSON("--inbox"); len(got) != 1 {
		t.Errorf("should be back in inbox")
	}
}

func TestExportImportRoundTrip(t *testing.T) {
	src := newCLI(t)
	src.mustRun("add", "task one", "-P", "uni", "-t", "x", "-n", "# Heading\nbody")
	pid := src.listJSON()[0].ShortID
	src.mustRun("add", "sub", "--parent", pid)
	src.mustRun("add", "done one")
	src.mustRun("done", src.listJSON()[1].ShortID)
	file := filepath.Join(t.TempDir(), "export.json")
	src.mustRun("export", "-o", file)

	dst := newCLI(t)
	dst.mustRun("add", "existing", "-P", "UNI") // same project name, different ID
	out := dst.mustRun("import", file)
	if !strings.Contains(out, "3 new") || !strings.Contains(out, "0 new projects") {
		t.Errorf("import summary: %s", out)
	}
	ts := dst.listJSON("-a")
	if len(ts) != 3 {
		t.Fatalf("imported list: %+v", ts)
	}
	for _, task := range ts {
		if task.Title == "task one" && (task.Project != "UNI" || task.Description != "# Heading\nbody" ||
			len(task.Subtasks) != 1) {
			t.Errorf("task one after import: %+v", task)
		}
	}
	// Re-importing is a no-op.
	if out := dst.mustRun("import", file); !strings.Contains(out, "0 new, 0 updated, 3 unchanged") {
		t.Errorf("re-import: %s", out)
	}
}

func TestErrors(t *testing.T) {
	c := newCLI(t)
	tests := []struct {
		args []string
		want string
	}{
		{[]string{"add", "x", "-p", "bogus"}, "invalid priority"},
		{[]string{"add", "x", "-d", "blursday"}, "could not understand date"},
		{[]string{"done", "deadbeef"}, "not found"},
		{[]string{"list", "--today", "--overdue"}, "none of the others"},
		{[]string{"list", "--project", "nope"}, "not found"},
	}
	for _, tt := range tests {
		_, err := c.run(tt.args...)
		if err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("tasker %v: err = %v, want %q", tt.args, err, tt.want)
		}
	}
}

func TestEditFileRoundTrip(t *testing.T) {
	in := editFields{Title: "T", Status: "todo", Priority: "high", Due: "2026-11-02 17:00",
		Project: "uni", Tags: "a, b", Description: "# Heading\n\n- item"}
	out, err := parseEditFile(renderEditFile(in))
	if err != nil {
		t.Fatal(err)
	}
	if out != in {
		t.Errorf("round trip:\n got %+v\nwant %+v", out, in)
	}
}

func TestParseEditFileErrors(t *testing.T) {
	tests := []struct{ name, in string }{
		{"no colon", "title T\n---\n"},
		{"unknown key", "colour: red\n---\n"},
	}
	for _, tt := range tests {
		if _, err := parseEditFile(tt.in); err == nil {
			t.Errorf("%s: expected error", tt.name)
		}
	}
}
