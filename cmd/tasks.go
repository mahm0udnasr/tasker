package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mahm0udnasr/tasker/internal/domain"
	"github.com/mahm0udnasr/tasker/internal/store"
)

func newAddCmd(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add <title...>",
		Short: "Add a task",
		Example: `  tasker add "Finish ML assignment" --due "tomorrow 8pm" --priority high --project uni --tag ml
  tasker add Buy milk -d today -t errands
  tasker add "Read chapter 3" --parent 3f2a9c1b`,
		Args: cobra.MinimumNArgs(1),
	}
	build := patchFlags(cmd, false)
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		s, err := a.Store(ctx)
		if err != nil {
			return err
		}
		t := domain.NewTask(strings.Join(args, " "), a.now())
		notes, err := a.applyPatch(ctx, s, &t, build())
		if err != nil {
			return err
		}
		if err := s.CreateTask(ctx, &t); err != nil {
			return err
		}
		for _, n := range notes {
			fmt.Fprintln(a.out, styles.dim.Render("  "+n))
		}
		return a.printResult(ctx, s, "Added", t)
	}
	return cmd
}

// printResult prints "✔ Verb <id> <title> <details>".
func (a *app) printResult(ctx context.Context, s store.Store, verb string, t domain.Task) error {
	projects, err := projectNames(ctx, s)
	if err != nil {
		return err
	}
	tbl := &table{}
	tbl.add(append([]string{styles.ok.Render("✔ " + verb)}, a.taskRow(t, projects, "")...)...)
	tbl.print(a.out)
	return nil
}

// taskRow renders the list columns for one task.
func (a *app) taskRow(t domain.Task, projects map[string]domain.Project, indent string) []string {
	proj := ""
	// Subtasks always share their parent's project, so skip the repetition.
	if p, ok := projects[t.ProjectID]; ok && !t.IsSubtask() {
		proj = styles.project.Render(strings.TrimSpace(p.Icon + " " + p.Name))
	}
	return []string{
		styles.id.Render(domain.ShortID(t.ID)),
		statusMark(t.Status),
		prioMark(t.Priority),
		titleCell(t, indent),
		dueBadge(t, a.now()),
		proj,
		tagChips(t.Tags),
	}
}

func newListCmd(a *app) *cobra.Command {
	var (
		views                     = map[domain.SmartView]*bool{}
		project, tag, sortBy      string
		all, asJSON, withSubtasks bool
	)
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List tasks",
		Example: `  tasker list --today
  tasker list --project uni --sort priority
  tasker list --overdue --json | jq '.[].title'`,
		Args: cobra.NoArgs,
	}
	for _, v := range domain.SmartViews {
		views[v] = new(bool)
		cmd.Flags().BoolVar(views[v], string(v), false, "show the "+v.Title()+" view")
	}
	cmd.MarkFlagsMutuallyExclusive("inbox", "today", "upcoming", "overdue", "completed")
	cmd.Flags().StringVarP(&project, "project", "P", "", "only tasks in this project")
	cmd.Flags().StringVarP(&tag, "tag", "t", "", "only tasks with this tag")
	cmd.Flags().StringVar(&sortBy, "sort", "", "manual, due, priority, created, title or completed")
	cmd.Flags().BoolVarP(&all, "all", "a", false, "include completed tasks")
	cmd.Flags().BoolVarP(&withSubtasks, "subtasks", "s", false, "show subtasks under their parents")
	cmd.Flags().BoolVar(&asJSON, "json", false, "print JSON")

	cmd.RunE = func(cmd *cobra.Command, _ []string) error {
		ctx := cmd.Context()
		s, err := a.Store(ctx)
		if err != nil {
			return err
		}
		f := domain.Filter{View: domain.ViewAll, Tag: tag, IncludeDone: all, Now: a.now()}
		for v, on := range views {
			if *on {
				f.View = v
			}
		}
		projects, err := projectNames(ctx, s)
		if err != nil {
			return err
		}
		if project != "" {
			p, err := s.GetProjectByName(ctx, project)
			if err != nil {
				if p, err = s.GetProject(ctx, project); err != nil {
					return fmt.Errorf("project %q not found", project)
				}
			}
			f.ProjectID = p.ID
		}
		key := domain.DefaultSort(f.View)
		if sortBy != "" {
			if key, err = domain.ParseSortKey(sortBy); err != nil {
				return err
			}
		}
		tasks, err := s.ListTasks(ctx, f, key)
		if err != nil {
			return err
		}

		subtasks := func(t domain.Task) ([]domain.Task, error) {
			if (!withSubtasks && !asJSON) || t.SubtasksTotal == 0 {
				return nil, nil
			}
			return s.ListTasks(ctx, domain.Filter{ParentID: t.ID, IncludeDone: true}, domain.SortManual)
		}

		if asJSON {
			out := make([]jsonTask, 0, len(tasks))
			for _, t := range tasks {
				subs, err := subtasks(t)
				if err != nil {
					return err
				}
				jt := a.toJSON(t, projects)
				for _, st := range subs {
					jt.Subtasks = append(jt.Subtasks, a.toJSON(st, projects))
				}
				out = append(out, jt)
			}
			return writeJSON(a.rawOut, out)
		}

		if len(tasks) == 0 {
			fmt.Fprintln(a.out, styles.dim.Render(emptyMessage(f.View)))
			return nil
		}
		tbl := &table{}
		for _, t := range tasks {
			tbl.add(a.taskRow(t, projects, "")...)
			subs, err := subtasks(t)
			if err != nil {
				return err
			}
			for i, st := range subs {
				branch := "├─ "
				if i == len(subs)-1 {
					branch = "└─ "
				}
				tbl.add(a.taskRow(st, projects, styles.dim.Render(branch))...)
			}
		}
		tbl.print(a.out)
		fmt.Fprintln(a.out)
		fmt.Fprintln(a.out, styles.dim.Render(fmt.Sprintf("%d %s", len(tasks), plural(len(tasks), "task"))))
		return nil
	}
	return cmd
}

func (a *app) toJSON(t domain.Task, projects map[string]domain.Project) jsonTask {
	return jsonTask{
		Task:         t,
		ShortID:      domain.ShortID(t.ID),
		FocusSeconds: int64(t.FocusTime.Seconds()),
		Project:      projects[t.ProjectID].Name,
		Overdue:      t.IsOverdue(a.now()),
	}
}

func emptyMessage(v domain.SmartView) string {
	switch v {
	case domain.ViewToday:
		return "Nothing due today. Enjoy the calm ☕"
	case domain.ViewOverdue:
		return "Nothing overdue. You're on top of it ✨"
	case domain.ViewCompleted:
		return "No completed tasks yet."
	case domain.ViewInbox:
		return "Inbox zero 🎉"
	}
	return `No tasks. Add one with: tasker add "Something to do"`
}

func plural(n int, word string) string {
	if n == 1 {
		return word
	}
	return word + "s"
}

func newShowCmd(a *app) *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "show <id>",
		Short: "Show a task's details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			s, err := a.Store(ctx)
			if err != nil {
				return err
			}
			t, err := s.GetTask(ctx, args[0])
			if err != nil {
				return err
			}
			projects, err := projectNames(ctx, s)
			if err != nil {
				return err
			}
			subs, err := s.ListTasks(ctx, domain.Filter{ParentID: t.ID, IncludeDone: true}, domain.SortManual)
			if err != nil {
				return err
			}
			if asJSON {
				jt := a.toJSON(t, projects)
				for _, st := range subs {
					jt.Subtasks = append(jt.Subtasks, a.toJSON(st, projects))
				}
				return writeJSON(a.rawOut, jt)
			}

			now := a.now()
			w := a.out
			fmt.Fprintln(w, statusMark(t.Status), styles.header.Render(t.Title), styles.id.Render(t.ID))
			field := func(k, v string) {
				if v != "" {
					fmt.Fprintf(w, "  %s %s\n", styles.dim.Render(fmt.Sprintf("%-9s", k)), v)
				}
			}
			field("status", t.Status.Label())
			if t.Priority != domain.PriorityNone {
				field("priority", prioMark(t.Priority)+" "+t.Priority.String())
			}
			field("due", dueBadge(t, now))
			if p, ok := projects[t.ProjectID]; ok {
				field("project", styles.project.Render(strings.TrimSpace(p.Icon+" "+p.Name)))
			} else {
				field("project", styles.dim.Render("Inbox"))
			}
			field("tags", tagChips(t.Tags))
			if t.Estimate > 0 || t.PomodorosDone > 0 || t.FocusTime > 0 {
				focus := fmt.Sprintf("🍅 %s pomodoros", t.PomodoroProgress())
				if t.PomodoroProgress() == "" {
					focus = "🍅 0 pomodoros"
				}
				if t.FocusTime > 0 {
					focus += " · " + domain.FormatFocus(t.FocusTime)
				}
				field("focus", focus)
			}
			field("created", t.CreatedAt.Format("Mon 2 Jan 2006 15:04"))
			if t.CompletedAt != nil {
				field("completed", t.CompletedAt.Format("Mon 2 Jan 2006 15:04"))
			}
			if len(subs) > 0 {
				fmt.Fprintln(w)
				fmt.Fprintln(w, styles.header.Render(fmt.Sprintf("Subtasks %s", t.SubtaskProgress())))
				tbl := &table{}
				for _, st := range subs {
					tbl.add(append([]string{" "}, a.taskRow(st, projects, "")...)...)
				}
				tbl.print(w)
			}
			if t.Description != "" {
				fmt.Fprintln(w)
				fmt.Fprintln(w, t.Description)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "print JSON")
	return cmd
}

func newProjectRmCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:     "rm <name|id>...",
		Aliases: []string{"delete"},
		Short:   "Delete projects; their tasks move to the Inbox",
		Args:    cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			s, err := a.Store(ctx)
			if err != nil {
				return err
			}
			for _, arg := range args {
				p, err := s.GetProjectByName(ctx, arg)
				if err != nil {
					if p, err = s.GetProject(ctx, arg); err != nil {
						return fmt.Errorf("project %q not found", arg)
					}
				}
				if err := s.DeleteProject(ctx, p.ID, a.now()); err != nil {
					return err
				}
				fmt.Fprintln(a.out, styles.err.Render("✘ Deleted project"), styles.project.Render(p.Name),
					styles.dim.Render("(tasks moved to Inbox)"))
			}
			return nil
		},
	}
}

// newStatusCmd builds done/reopen, which set a status on one or more tasks.
func newStatusCmd(a *app, use, short, verb string, status domain.Status) *cobra.Command {
	return &cobra.Command{
		Use:   use + " <id>...",
		Short: short,
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			s, err := a.Store(ctx)
			if err != nil {
				return err
			}
			for _, id := range args {
				t, err := s.GetTask(ctx, id)
				if err != nil {
					return err
				}
				t.SetStatus(status, a.now())
				if err := s.UpdateTask(ctx, &t); err != nil {
					return err
				}
				if err := a.printResult(ctx, s, verb, t); err != nil {
					return err
				}
			}
			return nil
		},
	}
}

func newDoneCmd(a *app) *cobra.Command {
	return newStatusCmd(a, "done", "Mark tasks as done", "Done", domain.StatusDone)
}

func newReopenCmd(a *app) *cobra.Command {
	return newStatusCmd(a, "reopen", "Mark done tasks as todo again", "Reopened", domain.StatusTodo)
}

func newRmCmd(a *app) *cobra.Command {
	return &cobra.Command{
		Use:     "rm <id>...",
		Aliases: []string{"delete"},
		Short:   "Delete tasks (and their subtasks)",
		Args:    cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			s, err := a.Store(ctx)
			if err != nil {
				return err
			}
			for _, id := range args {
				t, err := s.GetTask(ctx, id)
				if err != nil {
					return err
				}
				if err := s.DeleteTask(ctx, t.ID, a.now()); err != nil {
					return err
				}
				fmt.Fprintln(a.out, styles.err.Render("✘ Deleted"), styles.id.Render(domain.ShortID(t.ID)), t.Title)
			}
			return nil
		},
	}
}

func newEditCmd(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "edit <id>",
		Short: "Edit a task with flags, or in $EDITOR when no flags are given",
		Example: `  tasker edit 3f2a --due "next monday" --add-tag exam
  tasker edit 3f2a          # opens $EDITOR`,
		Args: cobra.ExactArgs(1),
	}
	build := patchFlags(cmd, true)
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		s, err := a.Store(ctx)
		if err != nil {
			return err
		}
		t, err := s.GetTask(ctx, args[0])
		if err != nil {
			return err
		}
		p := build()
		if p.empty() {
			var changed bool
			if p, changed, err = a.editInEditor(ctx, s, t); err != nil || !changed {
				return err
			}
		}
		notes, err := a.applyPatch(ctx, s, &t, p)
		if err != nil {
			return err
		}
		if err := s.UpdateTask(ctx, &t); err != nil {
			return err
		}
		for _, n := range notes {
			fmt.Fprintln(a.out, styles.dim.Render("  "+n))
		}
		return a.printResult(ctx, s, "Updated", t)
	}
	return cmd
}

// editInEditor round-trips the task through $VISUAL/$EDITOR.
func (a *app) editInEditor(ctx context.Context, s store.Store, t domain.Task) (taskPatch, bool, error) {
	projects, err := projectNames(ctx, s)
	if err != nil {
		return taskPatch{}, false, err
	}
	original := renderEditFile(editFields{
		Title:       t.Title,
		Status:      string(t.Status),
		Priority:    t.Priority.String(),
		Due:         formatDueForEdit(t.Due, t.DueHasTime),
		Project:     projects[t.ProjectID].Name,
		Tags:        strings.Join(t.Tags, ", "),
		Description: t.Description,
	})

	dir, err := os.MkdirTemp("", "tasker-edit-")
	if err != nil {
		return taskPatch{}, false, err
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, domain.ShortID(t.ID)+".md")
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		return taskPatch{}, false, err
	}

	editor := firstNonEmpty(os.Getenv("VISUAL"), os.Getenv("EDITOR"), "nano", "vi")
	parts := strings.Fields(editor)
	c := exec.CommandContext(ctx, parts[0], append(parts[1:], path)...) //nolint:gosec // user-chosen editor
	c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := c.Run(); err != nil {
		return taskPatch{}, false, fmt.Errorf("editor %q: %w", editor, err)
	}
	edited, err := os.ReadFile(path)
	if err != nil {
		return taskPatch{}, false, err
	}
	if string(edited) == original {
		fmt.Fprintln(a.out, styles.dim.Render("No changes."))
		return taskPatch{}, false, nil
	}
	f, err := parseEditFile(string(edited))
	if err != nil {
		return taskPatch{}, false, err
	}
	if f.Title == "" {
		fmt.Fprintln(a.out, styles.dim.Render("Title cleared — edit cancelled."))
		return taskPatch{}, false, nil
	}
	return f.patch(), true, nil
}

func firstNonEmpty(xs ...string) string {
	for _, x := range xs {
		if strings.TrimSpace(x) != "" {
			return x
		}
	}
	return ""
}

func newProjectsCmd(a *app) *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "projects",
		Short: "List projects with open task counts",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			s, err := a.Store(ctx)
			if err != nil {
				return err
			}
			ps, err := s.ListProjects(ctx)
			if err != nil {
				return err
			}
			if asJSON {
				return writeJSON(a.rawOut, ps)
			}
			c, err := s.Counts(ctx, a.now())
			if err != nil {
				return err
			}
			if len(ps) == 0 {
				fmt.Fprintln(a.out, styles.dim.Render("No projects yet. Use --project when adding a task."))
				return nil
			}
			tbl := &table{}
			for _, p := range ps {
				tbl.add(styles.id.Render(domain.ShortID(p.ID)),
					styles.project.Render(strings.TrimSpace(p.Icon+" "+p.Name)),
					styles.dim.Render(fmt.Sprintf("%d open", c.Projects[p.ID])))
			}
			tbl.print(a.out)
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "print JSON")
	cmd.AddCommand(newProjectRmCmd(a))
	return cmd
}
