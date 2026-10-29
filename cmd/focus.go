package cmd

import (
	"errors"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/mahm0udnasr/tasker/internal/domain"
	"github.com/mahm0udnasr/tasker/internal/store"
	"github.com/mahm0udnasr/tasker/internal/theme"
	"github.com/mahm0udnasr/tasker/internal/tui/common"
	"github.com/mahm0udnasr/tasker/internal/tui/pomodoro"
)

func newFocusCmd(a *app) *cobra.Command {
	var status, stop bool
	var minutes int
	cmd := &cobra.Command{
		Use:   "focus [id]",
		Short: "Run a pomodoro focus timer on a task",
		Long: "Opens a full-screen focus timer for the task. Press q to leave it\n" +
			"running in the background (the TUI shows it), X to stop it.\n" +
			"Without an ID, resumes the running session.",
		Example: `  tasker focus 3f2a
  tasker focus 3f2a --minutes 50
  tasker focus --status
  tasker focus --stop`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			s, err := a.Store(ctx)
			if err != nil {
				return err
			}
			active, err := s.ActivePomodoro(ctx)
			running := err == nil
			if err != nil && !errors.Is(err, store.ErrNotFound) {
				return err
			}

			switch {
			case status:
				if !running {
					fmt.Fprintln(a.out, styles.dim.Render("No focus session running."))
					return nil
				}
				t, _ := s.GetTask(ctx, active.TaskID)
				state := "running"
				if active.IsPaused() {
					state = "paused"
				}
				fmt.Fprintf(a.out, "🍅 %s left on %s (%s)\n",
					domain.FormatClock(active.Remaining(a.now())), styles.title.Render(t.Title), state)
				return nil
			case stop:
				if !running {
					return errors.New("no focus session running")
				}
				active.Finish(a.now())
				if err := s.UpdatePomodoro(ctx, &active); err != nil {
					return err
				}
				fmt.Fprintf(a.out, "%s %s of focus recorded\n", styles.ok.Render("✔ Stopped ·"),
					domain.FormatFocus(active.Focused(a.now())))
				return nil
			}

			var task domain.Task
			switch {
			case len(args) == 1:
				if task, err = s.GetTask(ctx, args[0]); err != nil {
					return err
				}
				if running && active.TaskID != task.ID {
					other, _ := s.GetTask(ctx, active.TaskID)
					return fmt.Errorf("a session is already running on “%s” — `tasker focus --stop` first", other.Title)
				}
			case running:
				if task, err = s.GetTask(ctx, active.TaskID); err != nil {
					return err
				}
			default:
				return errors.New("give a task ID to focus on")
			}

			opts := pomodoro.Options{Settings: a.cfg.Pomodoro.Settings(), Notify: a.cfg.Pomodoro.Notify, Bell: a.cfg.Pomodoro.Bell}
			if minutes > 0 {
				opts.Settings.Focus = time.Duration(min(minutes, 180)) * time.Minute
			}
			uiCtx := &common.Context{
				Styles: theme.NewStyles(theme.Get(a.cfg.Theme)),
				Icons:  theme.IconSet(a.cfg.Icons),
				Keys:   common.NewKeyMap(a.cfg.Keys),
				Now:    a.now,
			}
			return pomodoro.RunStandalone(uiCtx, s, opts, task, running)
		},
	}
	cmd.Flags().BoolVar(&status, "status", false, "print the running session and exit")
	cmd.Flags().BoolVar(&stop, "stop", false, "stop the running session (recorded as interrupted)")
	cmd.Flags().IntVarP(&minutes, "minutes", "m", 0, "focus length for this session (default from config)")
	cmd.MarkFlagsMutuallyExclusive("status", "stop")
	return cmd
}
