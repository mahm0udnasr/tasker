package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/mahm0udnasr/tasker/internal/domain"
	"github.com/mahm0udnasr/tasker/internal/store"
)

// exportVersion is bumped on incompatible changes to the export format.
const exportVersion = 1

type exportDoc struct {
	Version    int              `json:"version"`
	ExportedAt time.Time        `json:"exported_at"`
	Projects   []domain.Project `json:"projects"`
	Tasks      []domain.Task    `json:"tasks"`
}

func newExportCmd(a *app) *cobra.Command {
	var output string
	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export all tasks and projects as JSON",
		Example: `  tasker export > backup.json
  tasker export -o backup.json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			s, err := a.Store(ctx)
			if err != nil {
				return err
			}
			doc, err := buildExport(ctx, s, a.now())
			if err != nil {
				return err
			}
			if output == "" || output == "-" {
				return writeJSON(a.rawOut, doc)
			}
			f, err := os.OpenFile(output, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
			if err != nil {
				return err
			}
			if err := writeJSON(f, doc); err != nil {
				_ = f.Close()
				return err
			}
			if err := f.Close(); err != nil {
				return err
			}
			fmt.Fprintf(a.out, "%s %d tasks and %d projects to %s\n",
				styles.ok.Render("✔ Exported"), len(doc.Tasks), len(doc.Projects), output)
			return nil
		},
	}
	cmd.Flags().StringVarP(&output, "output", "o", "", "write to file instead of stdout")
	return cmd
}

func buildExport(ctx context.Context, s store.Store, now time.Time) (exportDoc, error) {
	ps, err := s.ListProjects(ctx)
	if err != nil {
		return exportDoc{}, err
	}
	ts, err := s.ListTasks(ctx, domain.Filter{IncludeSubtasks: true, IncludeDone: true}, domain.SortManual)
	if err != nil {
		return exportDoc{}, err
	}
	for i := range ts { // derived fields are recomputed on import
		ts[i].SubtasksTotal, ts[i].SubtasksDone = 0, 0
	}
	if ps == nil {
		ps = []domain.Project{}
	}
	if ts == nil {
		ts = []domain.Task{}
	}
	return exportDoc{Version: exportVersion, ExportedAt: now, Projects: ps, Tasks: ts}, nil
}

func newImportCmd(a *app) *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "import <file|->",
		Short: "Import tasks and projects from a JSON export",
		Long: "Import merges by ID: new records are added, and existing ones are\n" +
			"overwritten only when the imported copy is newer (last writer wins).\n" +
			"Projects whose name matches an existing project are merged into it.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			r := cmd.InOrStdin()
			if args[0] != "-" {
				f, err := os.Open(args[0])
				if err != nil {
					return err
				}
				defer f.Close()
				r = f
			}
			var doc exportDoc
			dec := json.NewDecoder(r)
			if err := dec.Decode(&doc); err != nil {
				return fmt.Errorf("parse %s: %w", args[0], err)
			}
			if doc.Version != exportVersion {
				return fmt.Errorf("unsupported export version %d (want %d)", doc.Version, exportVersion)
			}
			s, err := a.Store(ctx)
			if err != nil {
				return err
			}
			res, err := importDoc(ctx, s, doc, dryRun)
			if err != nil {
				return err
			}
			verb := "✔ Imported"
			if dryRun {
				verb = "Dry run:"
			}
			fmt.Fprintf(a.out, "%s %d new, %d updated, %d unchanged tasks; %d new projects\n",
				styles.ok.Render(verb), res.added, res.updated, res.skipped, res.projects)
			return nil
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "report what would change without writing")
	return cmd
}

type importResult struct{ added, updated, skipped, projects int }

func importDoc(ctx context.Context, s store.Store, doc exportDoc, dryRun bool) (importResult, error) {
	var res importResult

	// Map imported project IDs onto existing ones with the same name, so
	// importing an export from another machine doesn't duplicate projects.
	remap := map[string]string{}
	var newProjects []domain.Project
	for _, p := range doc.Projects {
		existing, err := s.GetProjectByName(ctx, p.Name)
		switch {
		case err == nil:
			remap[p.ID] = existing.ID
		case errors.Is(err, store.ErrNotFound):
			p.DeletedAt = nil
			newProjects = append(newProjects, p)
		default:
			return res, err
		}
	}
	res.projects = len(newProjects)

	var toWrite []domain.Task
	for _, t := range doc.Tasks {
		if id, ok := remap[t.ProjectID]; ok {
			t.ProjectID = id
		}
		if err := t.Validate(); err != nil {
			return res, fmt.Errorf("task %s: %w", domain.ShortID(t.ID), err)
		}
		existing, err := s.GetTask(ctx, t.ID)
		switch {
		case errors.Is(err, store.ErrNotFound):
			res.added++
		case err != nil:
			return res, err
		case !t.UpdatedAt.After(existing.UpdatedAt):
			res.skipped++
			continue
		default:
			res.updated++
		}
		toWrite = append(toWrite, t)
	}
	if dryRun {
		return res, nil
	}
	if err := s.RestoreProjects(ctx, newProjects); err != nil {
		return res, err
	}
	return res, s.RestoreTasks(ctx, toWrite)
}
