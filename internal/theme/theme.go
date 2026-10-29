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
	register(Theme{
		Name: "catppuccin-frappe", Dark: true,
		Base: c("#303446"), Surface: c("#414559"), Overlay: c("#292c3c"), Border: c("#51576d"),
		Text: c("#c6d0f5"), Subtext: c("#a5adce"), Muted: c("#737994"),
		Accent: c("#ca9ee6"), Secondary: c("#babbf1"), Tag: c("#81c8be"),
		Success: c("#a6d189"), Warning: c("#e5c890"), Danger: c("#e78284"), Info: c("#8caaee"),
		Priority: [5]color.Color{c("#626880"), c("#8caaee"), c("#e5c890"), c("#ef9f76"), c("#e78284")},
	})
	register(Theme{
		Name: "catppuccin-macchiato", Dark: true,
		Base: c("#24273a"), Surface: c("#363a4f"), Overlay: c("#1e2030"), Border: c("#494d64"),
		Text: c("#cad3f5"), Subtext: c("#a5adcb"), Muted: c("#6e738d"),
		Accent: c("#c6a0f6"), Secondary: c("#b7bdf8"), Tag: c("#8bd5ca"),
		Success: c("#a6da95"), Warning: c("#eed49f"), Danger: c("#ed8796"), Info: c("#8aadf4"),
		Priority: [5]color.Color{c("#5b6078"), c("#8aadf4"), c("#eed49f"), c("#f5a97f"), c("#ed8796")},
	})
	register(Theme{
		Name: "dracula", Dark: true,
		Base: c("#282a36"), Surface: c("#44475a"), Overlay: c("#21222c"), Border: c("#44475a"),
		Text: c("#f8f8f2"), Subtext: c("#d6d6d0"), Muted: c("#6272a4"),
		Accent: c("#bd93f9"), Secondary: c("#ff79c6"), Tag: c("#8be9fd"),
		Success: c("#50fa7b"), Warning: c("#f1fa8c"), Danger: c("#ff5555"), Info: c("#8be9fd"),
		Priority: [5]color.Color{c("#44475a"), c("#8be9fd"), c("#f1fa8c"), c("#ffb86c"), c("#ff5555")},
	})
	register(Theme{
		Name: "one-dark", Dark: true,
		Base: c("#282c34"), Surface: c("#3e4451"), Overlay: c("#21252b"), Border: c("#3e4451"),
		Text: c("#abb2bf"), Subtext: c("#9da5b4"), Muted: c("#5c6370"),
		Accent: c("#61afef"), Secondary: c("#c678dd"), Tag: c("#56b6c2"),
		Success: c("#98c379"), Warning: c("#e5c07b"), Danger: c("#e06c75"), Info: c("#61afef"),
		Priority: [5]color.Color{c("#4b5263"), c("#61afef"), c("#e5c07b"), c("#d19a66"), c("#e06c75")},
	})
	register(Theme{
		Name: "rose-pine", Dark: true,
		Base: c("#191724"), Surface: c("#403d52"), Overlay: c("#1f1d2e"), Border: c("#403d52"),
		Text: c("#e0def4"), Subtext: c("#908caa"), Muted: c("#6e6a86"),
		Accent: c("#c4a7e7"), Secondary: c("#9ccfd8"), Tag: c("#ebbcba"),
		Success: c("#9ccfd8"), Warning: c("#f6c177"), Danger: c("#eb6f92"), Info: c("#9ccfd8"),
		Priority: [5]color.Color{c("#403d52"), c("#9ccfd8"), c("#f6c177"), c("#ebbcba"), c("#eb6f92")},
	})
	register(Theme{
		Name: "rose-pine-dawn", Dark: false,
		Base: c("#faf4ed"), Surface: c("#dfdad9"), Overlay: c("#f2e9e1"), Border: c("#cecacd"),
		Text: c("#575279"), Subtext: c("#797593"), Muted: c("#9893a5"),
		Accent: c("#907aa9"), Secondary: c("#286983"), Tag: c("#56949f"),
		Success: c("#286983"), Warning: c("#ea9d34"), Danger: c("#b4637a"), Info: c("#56949f"),
		Priority: [5]color.Color{c("#cecacd"), c("#56949f"), c("#ea9d34"), c("#d7827e"), c("#b4637a")},
	})
	register(Theme{
		Name: "solarized-dark", Dark: true,
		Base: c("#002b36"), Surface: c("#073642"), Overlay: c("#00212b"), Border: c("#0a4a59"),
		Text: c("#93a1a1"), Subtext: c("#839496"), Muted: c("#586e75"),
		Accent: c("#268bd2"), Secondary: c("#6c71c4"), Tag: c("#2aa198"),
		Success: c("#859900"), Warning: c("#b58900"), Danger: c("#dc322f"), Info: c("#268bd2"),
		Priority: [5]color.Color{c("#586e75"), c("#268bd2"), c("#b58900"), c("#cb4b16"), c("#dc322f")},
	})
	register(Theme{
		Name: "solarized-light", Dark: false,
		Base: c("#fdf6e3"), Surface: c("#eee8d5"), Overlay: c("#eee8d5"), Border: c("#d9d2c2"),
		Text: c("#586e75"), Subtext: c("#657b83"), Muted: c("#93a1a1"),
		Accent: c("#268bd2"), Secondary: c("#6c71c4"), Tag: c("#2aa198"),
		Success: c("#859900"), Warning: c("#b58900"), Danger: c("#dc322f"), Info: c("#268bd2"),
		Priority: [5]color.Color{c("#93a1a1"), c("#268bd2"), c("#b58900"), c("#cb4b16"), c("#dc322f")},
	})
	register(Theme{
		Name: "kanagawa", Dark: true,
		Base: c("#1f1f28"), Surface: c("#2d4f67"), Overlay: c("#16161d"), Border: c("#363646"),
		Text: c("#dcd7ba"), Subtext: c("#c8c093"), Muted: c("#727169"),
		Accent: c("#7e9cd8"), Secondary: c("#957fb8"), Tag: c("#7aa89f"),
		Success: c("#98bb6c"), Warning: c("#e6c384"), Danger: c("#e46876"), Info: c("#7fb4ca"),
		Priority: [5]color.Color{c("#363646"), c("#7e9cd8"), c("#e6c384"), c("#ffa066"), c("#e46876")},
	})
	register(Theme{
		Name: "everforest", Dark: true,
		Base: c("#2d353b"), Surface: c("#3d484d"), Overlay: c("#272e33"), Border: c("#475258"),
		Text: c("#d3c6aa"), Subtext: c("#9da9a0"), Muted: c("#7a8478"),
		Accent: c("#a7c080"), Secondary: c("#7fbbb3"), Tag: c("#83c092"),
		Success: c("#a7c080"), Warning: c("#dbbc7f"), Danger: c("#e67e80"), Info: c("#7fbbb3"),
		Priority: [5]color.Color{c("#475258"), c("#7fbbb3"), c("#dbbc7f"), c("#e69875"), c("#e67e80")},
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
