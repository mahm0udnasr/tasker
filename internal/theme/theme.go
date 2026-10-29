// Package theme defines tasker's color palettes, the lipgloss styles derived
// from them, and the icon sets. Components never hard-code colors; they read
// semantic tokens (Accent, Muted, Danger...) so every theme just works.
package theme

import (
	"fmt"
	"image/color"
	"sort"

	"charm.land/lipgloss/v2"
)

// Theme is a palette of semantic color tokens.
type Theme struct {
	Name string
	Dark bool

	Base    color.Color // app background
	Surface color.Color // selected row, chips, raised panels
	Overlay color.Color // modal background
	Border  color.Color // unfocused borders
	Text    color.Color
	Subtext color.Color // secondary text
	Muted   color.Color // hints, done tasks, separators

	Accent    color.Color // focus, selection marker, headings
	Secondary color.Color // projects
	Tag       color.Color
	Success   color.Color
	Warning   color.Color // due today
	Danger    color.Color // overdue, delete
	Info      color.Color

	// Priority colors, indexed by domain.Priority (none..urgent).
	Priority [5]color.Color
}

var registry = map[string]Theme{}

func register(t Theme) { registry[t.Name] = t }

// Get returns a theme by name, falling back to the default.
func Get(name string) Theme {
	if t, ok := registry[name]; ok {
		return t
	}
	return registry[Default]
}

// Names lists the built-in themes alphabetically.
func Names() []string {
	out := make([]string, 0, len(registry))
	for n := range registry {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// Default is used when the configured theme is unknown.
const Default = "catppuccin-mocha"

func init() {
	c := lipgloss.Color
	register(Theme{
		Name: "catppuccin-mocha", Dark: true,
		Base: c("#1e1e2e"), Surface: c("#313244"), Overlay: c("#181825"), Border: c("#45475a"),
		Text: c("#cdd6f4"), Subtext: c("#a6adc8"), Muted: c("#6c7086"),
		Accent: c("#cba6f7"), Secondary: c("#b4befe"), Tag: c("#94e2d5"),
		Success: c("#a6e3a1"), Warning: c("#f9e2af"), Danger: c("#f38ba8"), Info: c("#89b4fa"),
		Priority: [5]color.Color{c("#585b70"), c("#89b4fa"), c("#f9e2af"), c("#fab387"), c("#f38ba8")},
	})
	register(Theme{
		Name: "catppuccin-latte", Dark: false,
		Base: c("#eff1f5"), Surface: c("#ccd0da"), Overlay: c("#e6e9ef"), Border: c("#bcc0cc"),
		Text: c("#4c4f69"), Subtext: c("#5c5f77"), Muted: c("#8c8fa1"),
		Accent: c("#8839ef"), Secondary: c("#7287fd"), Tag: c("#179299"),
		Success: c("#40a02b"), Warning: c("#df8e1d"), Danger: c("#d20f39"), Info: c("#1e66f5"),
		Priority: [5]color.Color{c("#bcc0cc"), c("#1e66f5"), c("#df8e1d"), c("#fe640b"), c("#d20f39")},
	})
	register(Theme{
		Name: "tokyo-night", Dark: true,
		Base: c("#1a1b26"), Surface: c("#292e42"), Overlay: c("#16161e"), Border: c("#3b4261"),
		Text: c("#c0caf5"), Subtext: c("#a9b1d6"), Muted: c("#565f89"),
		Accent: c("#7aa2f7"), Secondary: c("#bb9af7"), Tag: c("#7dcfff"),
		Success: c("#9ece6a"), Warning: c("#e0af68"), Danger: c("#f7768e"), Info: c("#2ac3de"),
		Priority: [5]color.Color{c("#414868"), c("#7aa2f7"), c("#e0af68"), c("#ff9e64"), c("#f7768e")},
	})
	register(Theme{
		Name: "gruvbox", Dark: true,
		Base: c("#282828"), Surface: c("#3c3836"), Overlay: c("#1d2021"), Border: c("#504945"),
		Text: c("#ebdbb2"), Subtext: c("#d5c4a1"), Muted: c("#928374"),
		Accent: c("#fe8019"), Secondary: c("#83a598"), Tag: c("#8ec07c"),
		Success: c("#b8bb26"), Warning: c("#fabd2f"), Danger: c("#fb4934"), Info: c("#83a598"),
		Priority: [5]color.Color{c("#504945"), c("#83a598"), c("#fabd2f"), c("#fe8019"), c("#fb4934")},
	})
	register(Theme{
		Name: "nord", Dark: true,
		Base: c("#2e3440"), Surface: c("#3b4252"), Overlay: c("#272c36"), Border: c("#4c566a"),
		Text: c("#eceff4"), Subtext: c("#d8dee9"), Muted: c("#616e88"),
		Accent: c("#88c0d0"), Secondary: c("#81a1c1"), Tag: c("#8fbcbb"),
		Success: c("#a3be8c"), Warning: c("#ebcb8b"), Danger: c("#bf616a"), Info: c("#5e81ac"),
		Priority: [5]color.Color{c("#4c566a"), c("#81a1c1"), c("#ebcb8b"), c("#d08770"), c("#bf616a")},
	})
}

// Exists reports whether name is a built-in theme.
func Exists(name string) bool {
	_, ok := registry[name]
	return ok
}

// Hex renders a color as #rrggbb.
func Hex(c color.Color) string {
	r, g, b, _ := c.RGBA()
	return fmt.Sprintf("#%02x%02x%02x", r>>8, g>>8, b>>8)
}
