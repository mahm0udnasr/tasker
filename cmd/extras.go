package cmd

import (
	"fmt"
	"image/color"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/spf13/cobra"

	"github.com/mahm0udnasr/tasker/internal/config"
	"github.com/mahm0udnasr/tasker/internal/domain"
	"github.com/mahm0udnasr/tasker/internal/theme"
)

func newThemesCmd(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "themes",
		Short: "List themes (or `themes set <name>` to choose one)",
		Args:  cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			current := theme.Get(a.cfg.Theme).Name
			nameW := 0
			for _, name := range theme.Names() {
				nameW = max(nameW, len(name))
			}
			for _, name := range theme.Names() {
				t := theme.Get(name)
				var swatch strings.Builder
				for _, c := range []color.Color{t.Base, t.Accent, t.Secondary, t.Tag, t.Success, t.Warning, t.Danger} {
					swatch.WriteString(lipgloss.NewStyle().Background(c).Render("  "))
				}
				marker := "  "
				if name == current {
					marker = styles.ok.Render("● ")
				}
				kind := "dark"
				if !t.Dark {
					kind = "light"
				}
				fmt.Fprintf(a.out, "%s%-*s  %s  %s\n", marker, nameW, name, swatch.String(), styles.dim.Render(kind))
			}
			return nil
		},
	}
	cmd.AddCommand(&cobra.Command{
		Use:       "set <name>",
		Short:     "Set the theme in config.toml",
		Args:      cobra.ExactArgs(1),
		ValidArgs: theme.Names(),
		RunE: func(_ *cobra.Command, args []string) error {
			if !theme.Exists(args[0]) {
				return fmt.Errorf("unknown theme %q (have: %s)", args[0], strings.Join(theme.Names(), ", "))
			}
			if err := config.SetValue(a.cfgPath, "theme", args[0]); err != nil {
				return err
			}
			styles = newCLIStyles(theme.Get(args[0]))
			fmt.Fprintln(a.out, styles.ok.Render("✔ Theme set to"), args[0])
			return nil
		},
	})
	return cmd
}

func newStatsCmd(a *app) *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Show this week's productivity stats",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			s, err := a.Store(ctx)
			if err != nil {
				return err
			}
			now := a.now()
			const days = 7
			st, err := s.Stats(ctx, now, days)
			if err != nil {
				return err
			}
			sessions, err := s.ListPomodoros(ctx, domain.StartOfDay(now).AddDate(0, 0, -days+1), now.Add(time.Hour))
			if err != nil {
				return err
			}
			focus := domain.ComputeFocus(sessions, days, now)
			if asJSON {
				type day struct {
					Day          string `json:"day"`
					Completed    int    `json:"completed"`
					FocusMinutes int    `json:"focus_minutes"`
					Pomodoros    int    `json:"pomodoros"`
				}
				out := struct {
					Streak         int     `json:"streak"`
					BestStreak     int     `json:"best_streak"`
					CompletionRate float64 `json:"completion_rate"`
					Days           []day   `json:"days"`
				}{Streak: st.Streak, BestStreak: st.BestStreak, CompletionRate: st.CompletionRate}
				for i := range days {
					out.Days = append(out.Days, day{
						Day: st.PerDay[i].Day.Format("2006-01-02"), Completed: st.PerDay[i].Count,
						FocusMinutes: int(focus.PerDay[i].Focus.Minutes()), Pomodoros: focus.PerDay[i].Pomodoros,
					})
				}
				return writeJSON(a.rawOut, out)
			}

			peak := 1
			for _, d := range st.PerDay {
				peak = max(peak, d.Count)
			}
			fmt.Fprintln(a.out, styles.header.Render("This week"))
			for i, d := range st.PerDay {
				bar := strings.Repeat("█", d.Count*24/peak)
				if d.Count > 0 && bar == "" {
					bar = "▏"
				}
				label := d.Day.Format("Mon 2 Jan")
				f := ""
				if fd := focus.PerDay[i]; fd.Focus > 0 {
					f = styles.dim.Render(fmt.Sprintf("  🍅 %s", domain.FormatFocus(fd.Focus)))
				}
				fmt.Fprintf(a.out, "  %s  %s %s%s\n", styles.dim.Render(label), styles.ok.Render(bar),
					styles.title.Render(fmt.Sprint(d.Count)), f)
			}
			fmt.Fprintln(a.out)
			fmt.Fprintf(a.out, "  %s streak %s · best %s · %.0f%% complete · %s focus\n",
				styles.warn.Render("🔥"), styles.title.Render(fmt.Sprintf("%d", st.Streak)),
				fmt.Sprint(st.BestStreak), st.CompletionRate*100, domain.FormatFocus(focus.Total))
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "print JSON")
	return cmd
}
