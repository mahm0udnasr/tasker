package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/mahm0udnasr/tasker/internal/domain"
)

// CLI palette (Catppuccin Mocha). The writer downsamples it for the terminal
// and strips it entirely when output is piped or NO_COLOR is set.
var styles = struct {
	dim, id, title, done, ok, err, warn, overdue, today, tag, project, header lipgloss.Style
	prio                                                                      [5]lipgloss.Style
}{
	dim:     lipgloss.NewStyle().Foreground(lipgloss.Color("#6c7086")),
	id:      lipgloss.NewStyle().Foreground(lipgloss.Color("#7f849c")),
	title:   lipgloss.NewStyle().Foreground(lipgloss.Color("#cdd6f4")),
	done:    lipgloss.NewStyle().Foreground(lipgloss.Color("#6c7086")).Strikethrough(true),
	ok:      lipgloss.NewStyle().Foreground(lipgloss.Color("#a6e3a1")).Bold(true),
	err:     lipgloss.NewStyle().Foreground(lipgloss.Color("#f38ba8")).Bold(true),
	warn:    lipgloss.NewStyle().Foreground(lipgloss.Color("#f9e2af")),
	overdue: lipgloss.NewStyle().Foreground(lipgloss.Color("#f38ba8")).Bold(true),
	today:   lipgloss.NewStyle().Foreground(lipgloss.Color("#f9e2af")).Bold(true),
	tag:     lipgloss.NewStyle().Foreground(lipgloss.Color("#94e2d5")),
	project: lipgloss.NewStyle().Foreground(lipgloss.Color("#b4befe")),
	header:  lipgloss.NewStyle().Foreground(lipgloss.Color("#cba6f7")).Bold(true),
	prio: [5]lipgloss.Style{
		lipgloss.NewStyle().Foreground(lipgloss.Color("#45475a")),
		lipgloss.NewStyle().Foreground(lipgloss.Color("#89b4fa")),
		lipgloss.NewStyle().Foreground(lipgloss.Color("#f9e2af")),
		lipgloss.NewStyle().Foreground(lipgloss.Color("#fab387")),
		lipgloss.NewStyle().Foreground(lipgloss.Color("#f38ba8")).Bold(true),
	},
}

const maxTitleWidth = 60

// statusMark renders the checkbox column.
func statusMark(s domain.Status) string {
	switch s {
	case domain.StatusDone:
		return styles.ok.Render("✔")
	case domain.StatusInProgress:
		return styles.warn.Render("◐")
	default:
		return styles.dim.Render("○")
	}
}

func prioMark(p domain.Priority) string {
	if !p.Valid() {
		p = domain.PriorityNone
	}
	marks := [...]string{"·", "!", "!!", "!!!", "!!!!"}
	return styles.prio[p].Render(marks[p])
}

// dueBadge renders the due date: red when overdue, yellow for today.
func dueBadge(t domain.Task, now time.Time) string {
	if t.Due == nil {
		return ""
	}
	s := domain.FormatDue(*t.Due, t.DueHasTime, now)
	switch {
	case t.IsDone():
		return styles.dim.Render(s)
	case t.IsOverdue(now):
		return styles.overdue.Render(s)
	case t.IsDueToday(now):
		return styles.today.Render(s)
	}
	return styles.dim.Render(s)
}

func tagChips(tags []string) string {
	parts := make([]string, len(tags))
	for i, t := range tags {
		parts[i] = styles.tag.Render("#" + t)
	}
	return strings.Join(parts, " ")
}

func titleCell(t domain.Task, indent string) string {
	title := ansi.Truncate(t.Title, maxTitleWidth-lipgloss.Width(indent), "…")
	st := styles.title
	if t.IsDone() {
		st = styles.done
	}
	out := indent + st.Render(title)
	if p := t.SubtaskProgress(); p != "" {
		out += " " + styles.dim.Render("["+p+"]")
	}
	return out
}

// table prints left-aligned columns measured with ANSI-aware widths; empty
// trailing cells don't produce trailing spaces.
type table struct{ rows [][]string }

func (t *table) add(cells ...string) { t.rows = append(t.rows, cells) }

func (t *table) print(w io.Writer) {
	var widths []int
	for _, r := range t.rows {
		for i, c := range r {
			if i >= len(widths) {
				widths = append(widths, 0)
			}
			widths[i] = max(widths[i], lipgloss.Width(c))
		}
	}
	for _, r := range t.rows {
		var b strings.Builder
		last := len(r) - 1
		for last >= 0 && r[last] == "" {
			last--
		}
		for i := 0; i <= last; i++ {
			b.WriteString(r[i])
			if i < last {
				b.WriteString(strings.Repeat(" ", widths[i]-lipgloss.Width(r[i])+2))
			}
		}
		fmt.Fprintln(w, b.String())
	}
}

// jsonTask is the machine-readable task shape for --json output.
type jsonTask struct {
	domain.Task
	ShortID  string     `json:"short_id"`
	Project  string     `json:"project,omitempty"`
	Overdue  bool       `json:"overdue,omitempty"`
	Subtasks []jsonTask `json:"subtasks,omitempty"`
}

func writeJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
