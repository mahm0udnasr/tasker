-- Daily habits and their completion history. A log row means the habit was
-- done on `day`, a local calendar date (YYYY-MM-DD); undoing a day sets
-- deleted_at. Streaks are derived from the logs, never stored.

CREATE TABLE habits (
  id          TEXT PRIMARY KEY,
  name        TEXT NOT NULL,
  created_at  INTEGER NOT NULL,
  updated_at  INTEGER NOT NULL,
  deleted_at  INTEGER
);
CREATE UNIQUE INDEX ux_habits_name ON habits(name COLLATE NOCASE) WHERE deleted_at IS NULL;
CREATE INDEX ix_habits_updated ON habits(updated_at);

CREATE TABLE habit_logs (
  habit_id    TEXT NOT NULL REFERENCES habits(id),
  day         TEXT NOT NULL,
  created_at  INTEGER NOT NULL,
  updated_at  INTEGER NOT NULL,
  deleted_at  INTEGER,
  PRIMARY KEY (habit_id, day)
);
CREATE INDEX ix_habit_logs_updated ON habit_logs(updated_at);
