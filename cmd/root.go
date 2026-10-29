// Package cmd implements tasker's command-line interface. Commands are thin:
// they parse flags, call the store, and print.
package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/charmbracelet/colorprofile"
	"github.com/spf13/cobra"

	"github.com/mahm0udnasr/tasker/internal/config"
	"github.com/mahm0udnasr/tasker/internal/store"
	"github.com/mahm0udnasr/tasker/internal/store/sqlite"
	"github.com/mahm0udnasr/tasker/internal/theme"
)

// Version is set at build time via -ldflags.
var Version = "dev"

// nowFn is the clock; tests pin it to make relative dates deterministic.
var nowFn = time.Now

// app carries shared state for every command. The store is opened lazily so
// commands like `completion` and `version` never touch the database.
type app struct {
	cfgPath string
	dbPath  string
	cfg     config.Config
	cfgSeen bool // a config file existed on disk
	store   store.Store
	now     func() time.Time
	out     io.Writer // color-aware: downsamples or strips ANSI when not a TTY
	rawOut  io.Writer // for JSON and other machine-readable output
	errOut  io.Writer
}

// Store opens the database on first use.
func (a *app) Store(ctx context.Context) (store.Store, error) {
	if a.store != nil {
		return a.store, nil
	}
	path := a.dbPath
	if path == "" {
		var err error
		if path, err = a.cfg.DatabasePath(); err != nil {
			return nil, err
		}
	}
	s, err := sqlite.Open(ctx, path)
	if err != nil {
		return nil, err
	}
	a.store = s
	return s, nil
}

func (a *app) close() {
	if a.store != nil {
		_ = a.store.Close()
		a.store = nil
	}
}

// NewRootCmd builds the command tree. stdout/stderr are injectable for tests.
func NewRootCmd(stdout, stderr io.Writer) *cobra.Command {
	a := &app{
		now:    func() time.Time { return nowFn() },
		rawOut: stdout,
		out:    colorprofile.NewWriter(stdout, os.Environ()),
		errOut: colorprofile.NewWriter(stderr, os.Environ()),
	}

	root := &cobra.Command{
		Use:   "tasker",
		Short: "A fast, keyboard-driven task manager for the terminal",
		Long: "tasker is a keyboard-driven task manager. Run it without arguments to open\n" +
			"the TUI, or use the subcommands for quick capture and scripting.",
		Version:       Version,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			if a.cfgPath == "" {
				p, err := config.DefaultPath()
				if err != nil {
					return err
				}
				a.cfgPath = p
			}
			cfg, exists, err := config.Load(a.cfgPath)
			if err != nil {
				return err
			}
			a.cfg, a.cfgSeen = cfg, exists
			styles = newCLIStyles(theme.Get(cfg.Theme))
			return nil
		},
		PersistentPostRun: func(*cobra.Command, []string) { a.close() },
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.runTUI(cmd.Context())
		},
	}
	root.SetOut(stdout)
	root.SetErr(stderr)
	root.PersistentFlags().StringVar(&a.cfgPath, "config", "", "config file (default ~/.config/tasker/config.toml)")
	root.PersistentFlags().StringVar(&a.dbPath, "db", "", "database file (default ~/.local/share/tasker/tasker.db, or $TASKER_DB)")

	root.AddCommand(
		newAddCmd(a),
		newListCmd(a),
		newShowCmd(a),
		newDoneCmd(a),
		newReopenCmd(a),
		newRmCmd(a),
		newEditCmd(a),
		newProjectsCmd(a),
		newFocusCmd(a),
		newThemesCmd(a),
		newStatsCmd(a),
		newExportCmd(a),
		newImportCmd(a),
	)
	return root
}

// Execute runs the CLI and returns the process exit code.
func Execute() int {
	root := NewRootCmd(os.Stdout, os.Stderr)
	err := root.ExecuteContext(context.Background())
	if err == nil {
		return 0
	}
	w := colorprofile.NewWriter(os.Stderr, os.Environ())
	fmt.Fprintln(w, styles.err.Render("Error:"), friendlyError(err))
	return 1
}

// friendlyError rewrites store sentinel errors into actionable messages.
func friendlyError(err error) string {
	switch {
	case errors.Is(err, store.ErrAmbiguousID):
		return err.Error() + " — type more characters of the ID"
	case errors.Is(err, store.ErrNotFound):
		return err.Error() + " — run `tasker list` to see IDs"
	}
	return err.Error()
}
