package domain

import (
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode"
)

// Project groups tasks. Tasks without a project live in the Inbox.
type Project struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Color     string     `json:"color,omitempty"` // hex like "#89b4fa"; empty = theme default
	Icon      string     `json:"icon,omitempty"`  // usually a single emoji
	SortOrder float64    `json:"sort_order"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `json:"deleted_at,omitempty"`
}

// ProjectPalette is cycled through to give new projects distinct colors.
var ProjectPalette = []string{
	"#89b4fa", "#a6e3a1", "#f9e2af", "#f38ba8", "#cba6f7",
	"#94e2d5", "#fab387", "#74c7ec", "#f5c2e7", "#b4befe",
}

// NewProject builds a project; n is the number of existing projects and
// selects a default color.
func NewProject(name string, n int, now time.Time) Project {
	return Project{
		ID:        NewID(),
		Name:      strings.TrimSpace(name),
		Color:     ProjectPalette[n%len(ProjectPalette)],
		SortOrder: float64(n + 1),
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// Validate checks project invariants.
func (p *Project) Validate() error {
	p.Name = strings.TrimSpace(p.Name)
	switch {
	case p.Name == "":
		return fmt.Errorf("project name: %w", ErrEmptyTitle)
	case len([]rune(p.Name)) > 100:
		return fmt.Errorf("project name must be at most 100 characters")
	case p.Color != "" && !isHexColor(p.Color):
		return fmt.Errorf("project color %q must look like #rrggbb", p.Color)
	}
	return nil
}

func isHexColor(s string) bool {
	if len(s) != 7 || s[0] != '#' {
		return false
	}
	for _, r := range s[1:] {
		if !unicode.Is(unicode.ASCII_Hex_Digit, r) {
			return false
		}
	}
	return true
}

// NormalizeTag lowercases, trims a leading '#', and turns inner whitespace
// into dashes so "#Machine Learning" and "machine-learning" are one tag.
func NormalizeTag(in string) (string, error) {
	s := strings.ToLower(strings.TrimSpace(in))
	s = strings.TrimPrefix(s, "#")
	s = strings.Join(strings.Fields(s), "-")
	if s == "" || len(s) > 50 {
		return "", fmt.Errorf("%w: %q", ErrInvalidTagName, in)
	}
	for _, r := range s {
		if r == ',' || r == '#' || unicode.IsControl(r) {
			return "", fmt.Errorf("%w: %q", ErrInvalidTagName, in)
		}
	}
	return s, nil
}

// NormalizeTags normalizes, de-duplicates and sorts tags. Comma-separated
// entries ("a,b") are split, which makes CLI flags forgiving.
func NormalizeTags(in []string) ([]string, error) {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, raw := range in {
		for _, part := range strings.Split(raw, ",") {
			if strings.TrimSpace(part) == "" {
				continue
			}
			n, err := NormalizeTag(part)
			if err != nil {
				return nil, err
			}
			if _, ok := seen[n]; ok {
				continue
			}
			seen[n] = struct{}{}
			out = append(out, n)
		}
	}
	sort.Strings(out)
	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
}

// Tag is a label with its usage count, as listed in the sidebar.
type Tag struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}
