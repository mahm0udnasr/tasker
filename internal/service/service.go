// Package service holds use-case logic shared by the CLI and the TUI that
// spans several store calls.
package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/mahm0udnasr/tasker/internal/domain"
	"github.com/mahm0udnasr/tasker/internal/store"
)

// IsInbox reports whether a project name means "no project".
func IsInbox(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "", "inbox", "none":
		return true
	}
	return false
}

// ResolveProject finds a project by name (case-insensitive) or ID prefix,
// creating it by name when nothing matches. created reports whether a new
// project was made.
func ResolveProject(ctx context.Context, s store.Store, nameOrID string, now time.Time) (p domain.Project, created bool, err error) {
	p, err = s.GetProjectByName(ctx, nameOrID)
	if err == nil || !errors.Is(err, store.ErrNotFound) {
		return p, false, err
	}
	if p, err := s.GetProject(ctx, nameOrID); err == nil {
		return p, false, nil
	}
	existing, err := s.ListProjects(ctx)
	if err != nil {
		return p, false, err
	}
	p = domain.NewProject(nameOrID, len(existing), now)
	if err := s.CreateProject(ctx, &p); err != nil {
		return p, false, err
	}
	return p, true, nil
}
