package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// Template is the commented config written on first run, so every option is
// discoverable without reading the docs.
const Template = `# tasker configuration
# Docs: https://github.com/mahm0udnasr/tasker#configuration

# Theme: catppuccin-mocha, catppuccin-latte (light), tokyo-night, gruvbox, nord.
# Switch live in the app with T; the choice is saved here.
theme = "catppuccin-mocha"

# Icons: "nerd" needs a Nerd Font (https://www.nerdfonts.com); "ascii" works anywhere.
icons = "nerd"

# View opened at startup: inbox, today, upcoming, overdue, completed.
default_view = "today"

# Show the detail pane on wide terminals (toggle with v).
show_detail = true

# Database location (default: ~/.local/share/tasker/tasker.db, or $TASKER_DB).
# database = "~/Dropbox/tasker.db"

seen_welcome = true

[pomodoro]
focus_minutes = 25
short_break_minutes = 5
long_break_minutes = 15
long_break_every = 4
auto_start_breaks = true
auto_start_focus = false
notify = true    # desktop notification via notify-send
bell = true      # terminal bell

# Rebind any action. Names are listed in the help screen (?).
# [keys]
# add = ["a", "n"]
# toggle_done = ["x", "space"]
`

// WriteTemplate writes Template to path unless a file already exists.
func WriteTemplate(path string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	return os.WriteFile(path, []byte(Template), 0o600)
}

var sectionRe = regexp.MustCompile(`^\s*\[`)

// SetValue sets a top-level key (one that appears before any [section]) in
// the TOML file at path, editing that line in place so the user's comments
// and formatting survive. value must be a string or bool. A missing file is
// created from Template first.
func SetValue(path, key string, value any) error {
	var rendered string
	switch v := value.(type) {
	case string:
		rendered = strconv.Quote(v)
	case bool:
		rendered = strconv.FormatBool(v)
	default:
		return fmt.Errorf("unsupported config value type %T", value)
	}
	if err := WriteTemplate(path); err != nil {
		return err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read config: %w", err)
	}

	lines := strings.Split(string(data), "\n")
	keyRe := regexp.MustCompile(`^\s*` + regexp.QuoteMeta(key) + `\s*=`)
	firstSection := len(lines)
	for i, l := range lines {
		if sectionRe.MatchString(l) {
			firstSection = i
			break
		}
	}
	line := key + " = " + rendered
	replaced := false
	for i := 0; i < firstSection; i++ {
		if keyRe.MatchString(lines[i]) {
			lines[i] = line
			replaced = true
			break
		}
	}
	if !replaced {
		// Insert before the first section (or at the end), keeping a blank
		// line between the key and the section header.
		insert := []string{line}
		if firstSection < len(lines) {
			insert = append(insert, "")
		}
		lines = append(lines[:firstSection], append(insert, lines[firstSection:]...)...)
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), ".config-*.toml")
	if err != nil {
		return fmt.Errorf("save config: %w", err)
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.WriteString(strings.Join(lines, "\n")); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("save config: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("save config: %w", err)
	}
	return os.Rename(tmp.Name(), path)
}
