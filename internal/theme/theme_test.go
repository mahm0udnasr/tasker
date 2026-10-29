package theme

import (
	"image/color"
	"testing"

	"github.com/lucasb-eyer/go-colorful"
)

func TestBuiltinThemes(t *testing.T) {
	want := []string{"catppuccin-latte", "catppuccin-mocha", "gruvbox", "nord", "tokyo-night"}
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
}

// Text must stay readable on every theme: WCAG contrast of the main tokens
// against the background (7:1 for body text, 3:1 for UI accents).
func TestThemeContrast(t *testing.T) {
	for _, name := range Names() {
		th := Get(name)
		checks := []struct {
			label string
			c     color.Color
			min   float64
		}{
			{"text", th.Text, 7},
			{"subtext", th.Subtext, 4.5},
			{"accent", th.Accent, 3},
			{"danger", th.Danger, 3},
			{"warning", th.Warning, 2},
		}
		for _, ck := range checks {
			if r := contrast(Hex(th.Base), Hex(ck.c)); r < ck.min {
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
