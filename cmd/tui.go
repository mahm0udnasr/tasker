package cmd

import (
	"context"

	"github.com/mahm0udnasr/tasker/internal/tui"
)

// runTUI launches the interactive interface.
func (a *app) runTUI(ctx context.Context) error {
	s, err := a.Store(ctx)
	if err != nil {
		return err
	}
	return tui.Run(s, a.cfg)
}
