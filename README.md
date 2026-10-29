# tasker

A fast, keyboard-driven task manager for the terminal, built with Go and
[Charm](https://charm.sh). Single static binary, no CGO, data in SQLite.

> 🚧 Under active development: themes, the stats view and a first-run welcome
> are coming next.

## Install

```sh
# with Go 1.26+
go install github.com/mahm0udnasr/tasker@latest

# from source
git clone https://github.com/mahm0udnasr/tasker && cd tasker
make install        # → $(go env GOPATH)/bin/tasker
```

## TUI

Run `tasker` with no arguments. Press `?` for every shortcut.

| Key | Action | Key | Action |
|-----|--------|-----|--------|
| `j` / `k` | move down / up | `a` | add task |
| `g` / `G` | top / bottom | `A` | add subtask |
| `h` / `l`, `Tab` | switch panes | `e` | edit task |
| `Enter` | expand subtasks | `x` / `Space` | toggle done |
| `v` | toggle detail pane | `i` | toggle in progress |
| `/` | search all tasks | `d` | delete task |
| `Ctrl+P` or `:` | command palette | `p` | cycle priority |
| `f` / `F` | filter / clear filter | `t` | set due date |
| `s` | cycle sort | `m` | move to project |
| `u` | undo | `J` / `K` | reorder (manual sort) |
| `ctrl+r` | reload | `D` | delete selected project |
| `P` | start / pause pomodoro | `X` | stop pomodoro |
| `>` | skip to next phase | `z` | focus mode |
| `?` | help | `q` | quit |

- **Detail pane** (`v`): the selected task with metadata, subtasks and its notes
  rendered as markdown. On terminals narrower than 120 columns it opens as a
  modal instead.
- **Search** (`/`): fuzzy search over every task's title, tags and project;
  `Enter` jumps to the task (expanding its parent if it's a subtask).
- **Filter** (`f`): narrow the current list by priority, due date and status;
  the active filter is shown in the list title. `F` clears it.
- **Sort** (`s`): cycle manual → due → priority → created → title, remembered
  per view. In manual order, `J`/`K` move tasks.
- **Undo** (`u`): reverts the last change: completions, edits, moves, deletes
  (including a deleted project and its tasks), reorders. Up to 50 steps.
- **Command palette** (`Ctrl+P`): every action plus "Go to" any view, project
  or tag.

## Pomodoro

Select a task and press `P` to start a 25-minute focus session on it. The
countdown appears in the status bar and on the task's row (and in the
terminal's tab/taskbar progress indicator, where supported). Press `z` for
a distraction-free focus mode with a big clock.

- When a session ends you get a desktop notification (`notify-send`) and a
  terminal bell, and a 5-minute break starts; every 4th break is 15 minutes.
- `P` pauses/resumes, `>` skips to the next phase, `X` stops. A stopped
  session still counts toward the task's focus time; only full sessions
  count as pomodoros.
- Give a task an **estimate** (in the form, or `--estimate 4`) and its row
  shows `🍅 2/4`. Without one it just counts: `🍅 2`. The detail pane shows
  total focus time.
- The timer survives restarts: quit while a session runs and it's still
  running when you come back. If it ran out meanwhile, it's credited.

From the shell:

```sh
tasker focus 5d56              # full-screen timer; q leaves it running
tasker focus 5d56 --minutes 50 # one-off length
tasker focus --status          # 🍅 12:31 left on Write thesis (running)
tasker focus --stop
tasker add "Thesis chapter" --estimate 6
```

In the add/edit form: `Tab`/`Shift+Tab` move between fields, `Ctrl+S` saves,
`Esc` cancels. Adding from a view pre-fills it (e.g. due "today" in Today,
the project when a project is selected).

Every key can be rebound in `config.toml`:

```toml
[keys]
add = ["a", "n"]
toggle_done = ["x", "space", "c"]
```

## CLI quick start

```sh
tasker add "Finish ML assignment" --due "tomorrow 8pm" --priority high --project uni --tag ml
tasker add "Read paper" --parent 5d56          # subtask (any unique ID prefix works)
tasker list                    # open tasks
tasker list --today            # due today + overdue
tasker list --overdue --json   # machine-readable
tasker list --project uni -s   # with subtasks
tasker show 5d56
tasker done 5d56 a0a7          # several at once
tasker edit 5d56 --due "next monday" --add-tag exam
tasker edit 5d56               # opens $EDITOR
tasker rm 5d56
tasker projects                # list projects
tasker projects rm uni         # delete a project; its tasks move to Inbox
tasker export -o backup.json && tasker import backup.json
```

Due dates accept natural language (`today 5pm`, `tomorrow`, `friday`,
`next week`, `in 3 days`, `eod`) or `2026-12-24` / `2026-12-24 17:00`.

## Configuration

`~/.config/tasker/config.toml` (all keys optional):

```toml
theme = "catppuccin-mocha"
icons = "nerd"            # or "ascii" if you don't use a Nerd Font
default_view = "today"    # inbox, today, upcoming, overdue, completed
show_detail = true

[pomodoro]
focus_minutes = 25
short_break_minutes = 5
long_break_minutes = 15
long_break_every = 4
auto_start_breaks = true
auto_start_focus = false
notify = true             # desktop notifications via notify-send
bell = true               # terminal bell

[keys]                    # rebind any action; see `?` in the TUI for names
add = ["a", "n"]
```

## Files

| What     | Where                                                   |
|----------|---------------------------------------------------------|
| Config   | `$XDG_CONFIG_HOME/tasker/config.toml` (`~/.config/...`) |
| Database | `$XDG_DATA_HOME/tasker/tasker.db` (`~/.local/share/...`), or `$TASKER_DB`, or `--db` |

## Development

```sh
make build   # ./bin/tasker
make test    # unit + integration tests
make lint    # golangci-lint
make help    # all targets
```
