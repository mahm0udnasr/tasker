-- Pomodoro focus sessions, and an optional per-task estimate in pomodoros.

ALTER TABLE tasks ADD COLUMN estimate INTEGER NOT NULL DEFAULT 0 CHECK (estimate BETWEEN 0 AND 99);

CREATE TABLE pomodoros (
  id             TEXT PRIMARY KEY,
  task_id        TEXT NOT NULL REFERENCES tasks(id),
  started_at     INTEGER NOT NULL,
  length_ms      INTEGER NOT NULL,
  paused_at      INTEGER,                   -- set while paused
  paused_ms      INTEGER NOT NULL DEFAULT 0, -- total paused time before paused_at
  ended_at       INTEGER,                   -- NULL while running or paused
  completed      INTEGER NOT NULL DEFAULT 0, -- ran the full length
  created_at     INTEGER NOT NULL,
  updated_at     INTEGER NOT NULL,
  deleted_at     INTEGER
);
CREATE INDEX ix_pomodoros_task    ON pomodoros(task_id) WHERE deleted_at IS NULL;
CREATE INDEX ix_pomodoros_started ON pomodoros(started_at);
CREATE INDEX ix_pomodoros_active  ON pomodoros(ended_at) WHERE ended_at IS NULL;
CREATE INDEX ix_pomodoros_updated ON pomodoros(updated_at);
