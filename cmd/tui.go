package cmd

import (
	"context"
	"fmt"
)

// runTUI launches the interactive interface. Until the TUI lands (phase 2)
// it shows today's tasks.
func (a *app) runTUI(ctx context.Context) error {
	list := newListCmd(a)
	list.SetContext(ctx)
	if err := list.Flags().Set("today", "true"); err != nil {
		return err
	}
	fmt.Fprintln(a.out, styles.header.Render("Today"))
	return list.RunE(list, nil)
}
