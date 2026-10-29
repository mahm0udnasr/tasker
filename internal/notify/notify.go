// Package notify sends desktop notifications. On Linux it uses notify-send
// (libnotify), which ships with Ubuntu; when it's missing, notifications are
// silently skipped, since the in-app status bar shows the same information.
package notify

import (
	"context"
	"os/exec"
	"time"
)

// Send shows a desktop notification. It never blocks for long and never
// fails loudly: a notification is a nicety, not a requirement.
func Send(title, body string) error {
	path, err := exec.LookPath("notify-send")
	if err != nil {
		return nil //nolint:nilerr // no notifier installed: nothing to do
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	//nolint:gosec // fixed binary; title and body are passed as separate args, not via a shell
	return exec.CommandContext(ctx, path, "--app-name=tasker", "--icon=appointment-soon",
		"--expire-time=8000", title, body).Run()
}
