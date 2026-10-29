package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMissingReturnsDefaults(t *testing.T) {
	cfg, exists, err := Load(filepath.Join(t.TempDir(), "nope.toml"))
	if err != nil || exists {
		t.Fatalf("exists=%v err=%v", exists, err)
	}
	if cfg.Theme != Default().Theme {
		t.Errorf("theme = %q", cfg.Theme)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "config.toml")
	in := Default()
	in.Theme = "nord"
	in.Icons = IconsASCII
	in.Keys = map[string][]string{"add": {"n", "a"}}
	if err := Save(path, in); err != nil {
		t.Fatal(err)
	}
	out, exists, err := Load(path)
	if err != nil || !exists {
		t.Fatalf("exists=%v err=%v", exists, err)
	}
	if out.Theme != "nord" || out.Icons != IconsASCII || len(out.Keys["add"]) != 2 {
		t.Errorf("round trip = %+v", out)
	}
}

func TestLoadPartialKeepsDefaultsAndRejectsBadIcons(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(`icons = "emoji"`), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, _, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Icons != IconsNerd || cfg.Theme != Default().Theme {
		t.Errorf("cfg = %+v", cfg)
	}
}

func TestLoadInvalidTOML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	_ = os.WriteFile(path, []byte(`theme = `), 0o600)
	if _, _, err := Load(path); err == nil {
		t.Error("expected parse error")
	}
}

func TestPaths(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/cfg")
	t.Setenv("XDG_DATA_HOME", "relative/ignored")
	t.Setenv("TASKER_DB", "")
	if d, _ := Dir(); d != "/cfg/tasker" {
		t.Errorf("Dir = %q", d)
	}
	home, _ := os.UserHomeDir()
	if p, _ := Default().DatabasePath(); p != filepath.Join(home, ".local/share/tasker/tasker.db") {
		t.Errorf("DatabasePath = %q", p)
	}
	t.Setenv("TASKER_DB", "/tmp/x.db")
	if p, _ := Default().DatabasePath(); p != "/tmp/x.db" {
		t.Errorf("TASKER_DB ignored: %q", p)
	}
}

func TestPomodoroSettings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	_ = os.WriteFile(path, []byte("[pomodoro]\nfocus_minutes = 50\nshort_break_minutes = -3\nlong_break_every = 0\n"), 0o600)
	cfg, _, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	s := cfg.Pomodoro.Settings()
	if s.Focus.Minutes() != 50 || s.ShortBreak.Minutes() != 5 || s.LongEvery != 4 || s.LongBreak.Minutes() != 15 {
		t.Errorf("settings = %+v", s)
	}
	if !s.AutoBreaks || !cfg.Pomodoro.Notify { // untouched keys keep their defaults
		t.Errorf("defaults lost: %+v", cfg.Pomodoro)
	}
}
