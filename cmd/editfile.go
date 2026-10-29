package cmd

import (
	"bufio"
	"fmt"
	"strings"
	"time"
)

// The $EDITOR format: "key: value" header lines, a "---" separator, then the
// markdown description verbatim. '#' comment lines are only stripped from the
// header, so markdown headings in the description survive.
const editFileHelp = `# Edit the task, then save and quit. Lines starting with '#' above the
# '---' line are ignored. Clear the title to cancel.
#   status:   todo | in_progress | done
#   priority: none | low | medium | high | urgent
#   due:      2026-11-02, 2026-11-02 17:00, "tomorrow 5pm", or empty
#   project:  project name (created if missing), empty for Inbox
#   tags:     comma-separated
`

const editSeparator = "---"

type editFields struct {
	Title, Status, Priority, Due, Project, Tags, Description string
}

func formatDueForEdit(due *time.Time, hasTime bool) string {
	if due == nil {
		return ""
	}
	if hasTime {
		return due.Format("2006-01-02 15:04")
	}
	return due.Format("2006-01-02")
}

func renderEditFile(f editFields) string {
	var b strings.Builder
	b.WriteString(editFileHelp)
	fmt.Fprintf(&b, "title:    %s\n", f.Title)
	fmt.Fprintf(&b, "status:   %s\n", f.Status)
	fmt.Fprintf(&b, "priority: %s\n", f.Priority)
	fmt.Fprintf(&b, "due:      %s\n", f.Due)
	fmt.Fprintf(&b, "project:  %s\n", f.Project)
	fmt.Fprintf(&b, "tags:     %s\n", f.Tags)
	b.WriteString(editSeparator + "\n")
	b.WriteString(f.Description)
	if f.Description != "" && !strings.HasSuffix(f.Description, "\n") {
		b.WriteString("\n")
	}
	return b.String()
}

func parseEditFile(s string) (editFields, error) {
	var f editFields
	sc := bufio.NewScanner(strings.NewReader(s))
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	inBody := false
	var body []string
	for sc.Scan() {
		line := sc.Text()
		if inBody {
			body = append(body, line)
			continue
		}
		trimmed := strings.TrimSpace(line)
		switch {
		case trimmed == editSeparator:
			inBody = true
			continue
		case trimmed == "" || strings.HasPrefix(trimmed, "#"):
			continue
		}
		key, val, ok := strings.Cut(line, ":")
		if !ok {
			return f, fmt.Errorf("line %q: expected \"key: value\"", line)
		}
		val = strings.TrimSpace(val)
		switch strings.ToLower(strings.TrimSpace(key)) {
		case "title":
			f.Title = val
		case "status":
			f.Status = val
		case "priority":
			f.Priority = val
		case "due":
			f.Due = val
		case "project":
			f.Project = val
		case "tags":
			f.Tags = val
		default:
			return f, fmt.Errorf("unknown field %q", strings.TrimSpace(key))
		}
	}
	f.Description = strings.TrimSpace(strings.Join(body, "\n"))
	return f, sc.Err()
}

// patch converts edited fields into a full-replacement patch.
func (f editFields) patch() taskPatch {
	var tags []string
	for _, t := range strings.Split(f.Tags, ",") {
		if t = strings.TrimSpace(t); t != "" {
			tags = append(tags, t)
		}
	}
	return taskPatch{
		Title: &f.Title, Description: &f.Description, Status: &f.Status,
		Priority: &f.Priority, Due: &f.Due, Project: &f.Project, Tags: &tags,
	}
}
