// Package config loads and saves tasker's TOML configuration and resolves
// XDG-compliant file locations.
package config

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// AppName names the config and data directories.
const AppName = "tasker"

// Icon sets.
const (
	IconsNerd  = "nerd"
	IconsASCII = "ascii"
)

// Config is the user-editable configuration.
type Config struct {
	// Theme names a built-in theme (see `tasker themes`).
	Theme string `toml:"theme"`
	// Icons selects Nerd Font glyphs ("nerd") or a plain fallback ("ascii").
	Icons string `toml:"icons"`
	// Database overrides the database location.
	Database string `toml:"database,omitempty"`
	// DefaultView is the smart view opened at startup.
	DefaultView string `toml:"default_view"`
	// ShowDetail opens the detail pane by default on wide terminals.
	ShowDetail bool `toml:"show_detail"`
	// Keys maps action names to one or more keys, e.g. add = ["a"].
	// Missing actions fall back to the defaults.
	Keys map[string][]string `toml:"keys,omitempty"`
	// Seen records that the first-run welcome screen was shown.
	Seen bool `toml:"seen_welcome"`
}

// Default returns the configuration used when no file exists.
func Default() Config {
	return Config{
		Theme:       "catppuccin-mocha",
		Icons:       IconsNerd,
		DefaultView: "today",
		ShowDetail:  true,
	}
}

// Dir returns $XDG_CONFIG_HOME/tasker (default ~/.config/tasker).
func Dir() (string, error) {
	return xdgDir("XDG_CONFIG_HOME", ".config")
}

// DataDir returns $XDG_DATA_HOME/tasker (default ~/.local/share/tasker).
func DataDir() (string, error) {
	return xdgDir("XDG_DATA_HOME", filepath.Join(".local", "share"))
}

func xdgDir(env, fallback string) (string, error) {
	base := os.Getenv(env)
	if base == "" || !filepath.IsAbs(base) { // the spec says relative values must be ignored
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("locate home directory: %w", err)
		}
		base = filepath.Join(home, fallback)
	}
	return filepath.Join(base, AppName), nil
}

// DefaultPath is the config file location.
func DefaultPath() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.toml"), nil
}

// DatabasePath resolves the database: $TASKER_DB, then the config value,
// then $XDG_DATA_HOME/tasker/tasker.db.
func (c Config) DatabasePath() (string, error) {
	if p := os.Getenv("TASKER_DB"); p != "" {
		return p, nil
	}
	if c.Database != "" {
		return expandHome(c.Database), nil
	}
	dir, err := DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "tasker.db"), nil
}

func expandHome(p string) string {
	if len(p) > 1 && p[:2] == "~/" {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, p[2:])
		}
	}
	return p
}

// Load reads the config at path. A missing file yields Default() and
// exists=false; fields absent from the file keep their defaults.
func Load(path string) (cfg Config, exists bool, err error) {
	cfg = Default()
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return cfg, false, nil
	}
	if err != nil {
		return cfg, false, fmt.Errorf("read config: %w", err)
	}
	if _, err := toml.Decode(string(data), &cfg); err != nil {
		return Default(), true, fmt.Errorf("parse %s: %w", path, err)
	}
	if cfg.Icons != IconsNerd && cfg.Icons != IconsASCII {
		cfg.Icons = IconsNerd
	}
	return cfg, true, nil
}

// Save writes cfg atomically (temp file + rename) so a crash never leaves a
// truncated config behind.
func Save(path string, cfg Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	var buf bytes.Buffer
	buf.WriteString("# tasker configuration. See the README for every option.\n\n")
	if err := toml.NewEncoder(&buf).Encode(cfg); err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".config-*.toml")
	if err != nil {
		return fmt.Errorf("save config: %w", err)
	}
	defer os.Remove(tmp.Name()) // no-op after a successful rename
	if _, err := tmp.Write(buf.Bytes()); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("save config: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("save config: %w", err)
	}
	return os.Rename(tmp.Name(), path)
}
