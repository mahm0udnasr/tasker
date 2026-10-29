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
