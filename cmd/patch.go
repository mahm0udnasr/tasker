package cmd

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mahm0udnasr/tasker/internal/domain"
	"github.com/mahm0udnasr/tasker/internal/store"
)

// taskPatch is a set of optional field changes. nil means "leave unchanged",
// which lets add, edit-by-flags and edit-in-$EDITOR share one code path.
type taskPatch struct {
	Title       *string
	Description *string
	Status      *string
	Priority    *string
	Due         *string
	Project     *string // name or ID; "" moves to the Inbox
	Parent      *string // ID prefix; "" makes it top-level
	Tags        *[]string
	AddTags     []string
	RemoveTags  []string
}

func (p taskPatch) empty() bool {
	return p.Title == nil && p.Description == nil && p.Status == nil && p.Priority == nil &&
		p.Due == nil && p.Project == nil && p.Parent == nil && p.Tags == nil &&
		len(p.AddTags) == 0 && len(p.RemoveTags) == 0
}

// patchFlags binds the common field flags to a command and returns a
// function that builds the patch from the flags that were actually set.
func patchFlags(cmd *cobra.Command, withTitle bool) func() taskPatch {
	f := cmd.Flags()
	var (
		title, desc, status, prio, due, project, parent string
		tags, addTags, rmTags                           []string
	)
	if withTitle {
		f.StringVar(&title, "title", "", "new title")
	}
	f.StringVarP(&desc, "note", "n", "", "description (markdown)")
	f.StringVarP(&status, "status", "s", "", "todo, in_progress or done")
	f.StringVarP(&prio, "priority", "p", "", "none, low, medium, high or urgent")
	f.StringVarP(&due, "due", "d", "", `due date, e.g. "tomorrow 8pm", "next friday", "2026-12-24"; "none" clears`)
	f.StringVarP(&project, "project", "P", "", `project name (created if missing); "" moves to Inbox`)
	f.StringVar(&parent, "parent", "", "make this a subtask of the given task ID")
	f.StringSliceVarP(&tags, "tag", "t", nil, "tag (repeatable or comma-separated)")
	if withTitle { // edit-only flags
		f.StringSliceVar(&addTags, "add-tag", nil, "add tags, keeping existing ones")
		f.StringSliceVar(&rmTags, "rm-tag", nil, "remove tags")
	}

	return func() taskPatch {
		var p taskPatch
		set := func(name string, v *string) *string {
			if f.Changed(name) {
				return v
			}
			return nil
		}
		p.Title = set("title", &title)
		p.Description = set("note", &desc)
		p.Status = set("status", &status)
		p.Priority = set("priority", &prio)
		p.Due = set("due", &due)
		p.Project = set("project", &project)
		p.Parent = set("parent", &parent)
		if f.Changed("tag") {
			p.Tags = &tags
		}
		p.AddTags, p.RemoveTags = addTags, rmTags
		return p
	}
}

// applyPatch mutates t according to p. Projects named in the patch are
// created on demand; the returned notes describe such side effects.
func (a *app) applyPatch(ctx context.Context, s store.Store, t *domain.Task, p taskPatch) (notes []string, err error) {
	now := a.now()
	if p.Title != nil {
		t.Title = *p.Title
	}
	if p.Description != nil {
		t.Description = strings.TrimSpace(*p.Description)
	}
	if p.Status != nil {
		st, err := domain.ParseStatus(*p.Status)
		if err != nil {
			return nil, err
		}
		t.SetStatus(st, now)
	}
	if p.Priority != nil {
		pr, err := domain.ParsePriority(*p.Priority)
		if err != nil {
			return nil, err
		}
		t.Priority = pr
	}
	if p.Due != nil {
		due, hasTime, err := domain.ParseDue(*p.Due, now)
		if err != nil {
			return nil, err
		}
		t.SetDue(due, hasTime, now)
	}
	if p.Project != nil {
		name := strings.TrimSpace(*p.Project)
		switch strings.ToLower(name) {
		case "", "inbox", "none":
			t.ProjectID = ""
		default:
			proj, created, err := a.resolveProject(ctx, s, name)
			if err != nil {
				return nil, err
			}
			if created {
				notes = append(notes, "created project "+styles.project.Render(proj.Name))
			}
			t.ProjectID = proj.ID
		}
	}
	if p.Parent != nil {
		if *p.Parent == "" {
			t.ParentID = ""
		} else {
			parent, err := s.GetTask(ctx, *p.Parent)
			if err != nil {
				return nil, fmt.Errorf("parent: %w", err)
			}
			t.ParentID = parent.ID
		}
	}
	tags := t.Tags
	if p.Tags != nil {
		tags = *p.Tags
	}
	tags = append(append([]string(nil), tags...), p.AddTags...)
	if len(p.RemoveTags) > 0 {
		rm, err := domain.NormalizeTags(p.RemoveTags)
		if err != nil {
			return nil, err
		}
		norm, err := domain.NormalizeTags(tags)
		if err != nil {
			return nil, err
		}
		tags = tags[:0]
		for _, x := range norm {
			if !contains(rm, x) {
				tags = append(tags, x)
			}
		}
	}
	if err := t.SetTags(tags); err != nil {
		return nil, err
	}
	t.UpdatedAt = now
	return notes, nil
}

// resolveProject finds a project by name or ID prefix, creating it by name
// when nothing matches.
func (a *app) resolveProject(ctx context.Context, s store.Store, nameOrID string) (domain.Project, bool, error) {
	p, err := s.GetProjectByName(ctx, nameOrID)
	if err == nil {
		return p, false, nil
	}
	if !errors.Is(err, store.ErrNotFound) {
		return p, false, err
	}
	if p, err := s.GetProject(ctx, nameOrID); err == nil {
		return p, false, nil
	}
	existing, err := s.ListProjects(ctx)
	if err != nil {
		return p, false, err
	}
	p = domain.NewProject(nameOrID, len(existing), a.now())
	if err := s.CreateProject(ctx, &p); err != nil {
		return p, false, err
	}
	return p, true, nil
}

func contains(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

// projectNames maps project IDs to names for display.
func projectNames(ctx context.Context, s store.Store) (map[string]domain.Project, error) {
	ps, err := s.ListProjects(ctx)
	if err != nil {
		return nil, err
	}
	m := make(map[string]domain.Project, len(ps))
	for _, p := range ps {
		m[p.ID] = p
	}
	return m, nil
}
