// Package stats renders the productivity overview: streaks, completion
// rate, and bar charts of completions and focus time over the last week.
package stats

import (
	"fmt"
	"image/color"
	"math"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/mahm0udnasr/tasker/internal/domain"
	"github.com/mahm0udnasr/tasker/internal/tui/common"
)

// Days is the chart window.
const Days = 7

// Data is everything the view shows.
type Data struct {
	Tasks domain.Stats
	Focus domain.FocusStats
}

// Overlay is the stats modal.
type Overlay struct {
	ctx  *common.Context
	data Data
}

// New creates the stats overlay.
func New(ctx *common.Context, d Data) *Overlay { return &Overlay{ctx: ctx, data: d} }

// Update implements common.Overlay: esc, q or S closes.
func (o *Overlay) Update(msg tea.Msg) (common.Overlay, tea.Cmd) {
	if k, ok := msg.(tea.KeyPressMsg); ok {
		if k.String() == "esc" || k.String() == "q" || o.ctx.Keys.Is(k, common.ActStats) {
			return o, common.CloseOverlay
		}
	}
	return o, nil
}

// View implements common.Overlay.
func (o *Overlay) View() string {
	st := o.ctx.Styles
	th := st.Theme
	ic := o.ctx.Icons
	w := max(min(o.ctx.Width-8, 78), 40)
	d := o.data
	now := o.ctx.Now()

	weekDone := 0
	for _, x := range d.Tasks.PerDay {
		weekDone += x.Count
	}
	cards := []string{
		card(o.ctx, ic.Flame, plural(d.Tasks.Streak, "day"), "streak", th.Warning, w),
		card(o.ctx, ic.Done, fmt.Sprint(weekDone), "this week", th.Success, w),
		card(o.ctx, ic.Chart, fmt.Sprintf("%.0f%%", d.Tasks.CompletionRate*100), "completion", th.Info, w),
		card(o.ctx, ic.Tomato, domain.FormatFocus(d.Focus.Total), "focus time", th.Priority[4], w),
	}
	row := lipgloss.JoinHorizontal(lipgloss.Top, cards...)
	if lipgloss.Width(row) > w { // two by two on narrower terminals
		row = lipgloss.JoinVertical(lipgloss.Left,
			lipgloss.JoinHorizontal(lipgloss.Top, cards[0], cards[1]),
			lipgloss.JoinHorizontal(lipgloss.Top, cards[2], cards[3]))
	}

	labels := make([]string, Days)
	done := make([]float64, Days)
	focus := make([]float64, Days)
	for i := range Days {
		day := d.Tasks.PerDay[i].Day
		labels[i] = day.Format("Mon")
		if domain.SameDay(day, now) {
			labels[i] = "today"
		}
		done[i] = float64(d.Tasks.PerDay[i].Count)
		if i < len(d.Focus.PerDay) {
			focus[i] = d.Focus.PerDay[i].Focus.Minutes()
		}
	}
	colW := max(min((w-8)/Days, 9), 5)
	completed := barChart(o.ctx, done, labels, 6, colW, th.Success, func(v float64) string { return fmt.Sprint(int(v)) })
	focused := barChart(o.ctx, focus, labels, 6, colW, th.Priority[4], func(v float64) string {
		if v == 0 {
			return "0"
		}
		return domain.FormatFocus(time.Duration(v * float64(time.Minute)))
	})

	footer := fmt.Sprintf("best streak %s · %d of %d tasks done · %s %s this week",
		plural(d.Tasks.BestStreak, "day"), d.Tasks.Done, d.Tasks.Total, ic.Tomato, plural(d.Focus.Pomodoros, "pomodoro"))
	body := lipgloss.JoinVertical(lipgloss.Left,
		st.Bold.Render(ic.Chart+"  Your week")+"  "+st.Muted.Render(now.AddDate(0, 0, -Days+1).Format("2 Jan")+" – "+now.Format("2 Jan")),
		"",
		row,
		"",
		st.ModalTitle.Render("Tasks completed"),
		completed,
		"",
		st.ModalTitle.Render("Focus time"),
		focused,
		"",
		st.Muted.Render(footer),
		"",
		st.KeyDesc.Render("esc to close"),
	)
	return st.Modal.Padding(1, 2).Render(body)
}

func card(ctx *common.Context, icon, value, label string, c color.Color, total int) string {
	st := ctx.Styles
	// Four cards share the row; each also takes 2 border cells + 1 margin.
	w := max(min((total-12)/4, 20), 15)
	return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(st.Theme.Border).
		Width(w).Padding(0, 1).MarginRight(1).Render(
		lipgloss.NewStyle().Foreground(c).Bold(true).Render(icon+" "+value) + "\n" + st.Muted.Render(label))
}

// eighths are the partial-block characters for sub-cell bar heights.
var eighths = []string{" ", "▁", "▂", "▃", "▄", "▅", "▆", "▇", "█"}

// barChart draws vertical bars height rows tall, one column per value, with
// the value above each bar and labels underneath. The last column (today)
// is drawn in the accent color.
func barChart(ctx *common.Context, values []float64, labels []string, height, colW int, c color.Color,
	format func(float64) string) string {
	st := ctx.Styles
	peak := 0.0
	for _, v := range values {
		peak = math.Max(peak, v)
	}
	barW := max(colW-2, 1)
	center := func(s string, w int) string {
		gap := max(w-lipgloss.Width(s), 0)
		return strings.Repeat(" ", gap/2) + s + strings.Repeat(" ", gap-gap/2)
	}

	// Height of each bar in eighths of a row.
	units := make([]int, len(values))
	for i, v := range values {
		if peak > 0 {
			units[i] = int(math.Round(v / peak * float64(height*8)))
			if v > 0 && units[i] == 0 {
				units[i] = 1 // never hide a non-zero day entirely
			}
		}
	}

	// Row 1 is the bottom; each bar fills rows 1..top (the top row possibly
	// partially) and its value label sits on row top+1.
	var rows []string
	for r := height + 1; r >= 1; r-- {
		var b strings.Builder
		for i := range values {
			style := lipgloss.NewStyle().Foreground(c)
			if i == len(values)-1 {
				style = lipgloss.NewStyle().Foreground(st.Theme.Accent)
			}
			top := (units[i] + 7) / 8
			pad := strings.Repeat(" ", colW-barW-1)
			switch {
			case r == top+1:
				label := ""
				if values[i] > 0 || i == len(values)-1 {
					label = format(values[i])
				}
				b.WriteString(st.Subtext.Render(center(label, colW)))
			case r < top || (r == top && units[i]%8 == 0):
				b.WriteString(" " + style.Render(strings.Repeat("█", barW)) + pad)
			case r == top:
				b.WriteString(" " + style.Render(strings.Repeat(eighths[units[i]%8], barW)) + pad)
			default:
				b.WriteString(strings.Repeat(" ", colW))
			}
		}
		rows = append(rows, b.String())
	}
	baseline := st.Muted.Render(strings.Repeat("─", colW*len(values)))
	var lb strings.Builder
	for i, l := range labels {
		s := st.Muted
		if i == len(labels)-1 {
			s = st.Accent.Bold(true)
		}
		lb.WriteString(s.Render(center(l, colW)))
	}
	return strings.Join(append(rows, baseline, lb.String()), "\n")
}

func plural(n int, word string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, word)
	}
	return fmt.Sprintf("%d %ss", n, word)
}
