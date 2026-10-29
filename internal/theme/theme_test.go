package theme

import (
	"image/color"
	"testing"

	"github.com/lucasb-eyer/go-colorful"
)

func TestBuiltinThemes(t *testing.T) {
	want := []string{
		"catppuccin-frappe", "catppuccin-latte", "catppuccin-macchiato", "catppuccin-mocha",
		"dracula", "everforest", "gruvbox", "kanagawa", "nord", "one-dark",
		"rose-pine", "rose-pine-dawn", "solarized-dark", "solarized-light", "tokyo-night",
	}
	got := Names()
	if len(got) != len(want) {
		t.Fatalf("themes = %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("theme %d = %q, want %q", i, got[i], want[i])
		}
	}
	if Get("nope").Name != Default {
		t.Error("unknown theme should fall back to the default")
	}
	light := 0
	for _, n := range got {
		if !Get(n).Dark {
			light++
		}
	}
	if light != 3 {
		t.Errorf("light themes = %d, want 3 (latte, rose-pine-dawn, solarized-light)", light)
	}
}

// Text must stay readable on every theme. Body text meets WCAG AA (4.5:1);
// the stricter AAA level (7:1) would exclude classic palettes such as
// Solarized and One Dark, whose official foregrounds are AA. Secondary text
// and UI accents need 3:1 (WCAG's bar for large text and UI components).
func TestThemeContrast(t *testing.T) {
	for _, name := range Names() {
		th := Get(name)
		checks := []struct {
			label  string
			bg, fg color.Color
			min    float64
		}{
			{"text", th.Base, th.Text, 4.5},
			{"text on selected row", th.Surface, th.Text, 4},
			{"subtext", th.Base, th.Subtext, 3},
			{"accent", th.Base, th.Accent, 3},
			{"danger", th.Base, th.Danger, 3},
			{"warning", th.Base, th.Warning, 2},
		}
		for _, ck := range checks {
			if r := contrast(Hex(ck.bg), Hex(ck.fg)); r < ck.min {
				t.Errorf("%s: %s contrast %.2f < %.1f", name, ck.label, r, ck.min)
			}
		}
	}
}

func contrast(a, b string) float64 {
	ca, _ := colorful.Hex(a)
	cb, _ := colorful.Hex(b)
	la, lb := luminance(ca), luminance(cb)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

func luminance(c colorful.Color) float64 {
	r, g, b := c.LinearRgb()
	return 0.2126*r + 0.7152*g + 0.0722*b
}
