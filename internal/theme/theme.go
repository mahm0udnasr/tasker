// Package theme defines tasker's color palettes, the lipgloss styles derived
// from them, and the icon sets. Components never hard-code colors; they read
// semantic tokens (Accent, Muted, Danger...) so every theme just works.
package theme

import (
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
}
