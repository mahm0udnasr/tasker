# tasker

A fast, keyboard-driven task manager for the terminal, built with Go and
[Charm](https://charm.sh). Single static binary, no CGO, data in SQLite.

> 🚧 Under active development. The CLI and storage are complete; the full-screen
> TUI is landing next.

## Install

```sh
# with Go 1.26+
go install github.com/mahm0udnasr/tasker@latest

# from source
git clone https://github.com/mahm0udnasr/tasker && cd tasker
make install        # → $(go env GOPATH)/bin/tasker
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
