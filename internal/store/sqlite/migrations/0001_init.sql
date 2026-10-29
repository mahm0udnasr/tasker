-- All timestamps are unix milliseconds (UTC instants). Rows are never removed:
-- deleted_at marks a tombstone so a future sync engine can propagate deletes.

CREATE TABLE projects (
  id          TEXT PRIMARY KEY,
  name        TEXT NOT NULL,
  color       TEXT NOT NULL DEFAULT '',
  icon        TEXT NOT NULL DEFAULT '',
  sort_order  REAL NOT NULL DEFAULT 0,
  created_at  INTEGER NOT NULL,
  updated_at  INTEGER NOT NULL,
  deleted_at  INTEGER
);
CREATE UNIQUE INDEX ux_projects_name ON projects(name COLLATE NOCASE) WHERE deleted_at IS NULL;
CREATE INDEX ix_projects_updated ON projects(updated_at);

CREATE TABLE tasks (
  id           TEXT PRIMARY KEY,
  parent_id    TEXT REFERENCES tasks(id),
  project_id   TEXT REFERENCES projects(id),
  title        TEXT NOT NULL,
  description  TEXT NOT NULL DEFAULT '',
  status       TEXT NOT NULL DEFAULT 'todo' CHECK (status IN ('todo', 'in_progress', 'done')),
  priority     INTEGER NOT NULL DEFAULT 0 CHECK (priority BETWEEN 0 AND 4),
  due_at       INTEGER,
  due_has_time INTEGER NOT NULL DEFAULT 0,
  sort_order   REAL NOT NULL DEFAULT 0,
  created_at   INTEGER NOT NULL,
  updated_at   INTEGER NOT NULL,
  completed_at INTEGER,
  deleted_at   INTEGER
);
CREATE INDEX ix_tasks_live    ON tasks(status, due_at) WHERE deleted_at IS NULL;
CREATE INDEX ix_tasks_project ON tasks(project_id) WHERE deleted_at IS NULL;
CREATE INDEX ix_tasks_parent  ON tasks(parent_id);
CREATE INDEX ix_tasks_updated ON tasks(updated_at);

CREATE TABLE tags (
  id         TEXT PRIMARY KEY,
  name       TEXT NOT NULL,
  color      TEXT NOT NULL DEFAULT '',
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  deleted_at INTEGER
);
CREATE UNIQUE INDEX ux_tags_name ON tags(name COLLATE NOCASE) WHERE deleted_at IS NULL;

-- Changing a task's tags bumps tasks.updated_at, so sync can treat the tag set
-- as part of the task record.
CREATE TABLE task_tags (
  task_id TEXT NOT NULL REFERENCES tasks(id),
  tag_id  TEXT NOT NULL REFERENCES tags(id),
  PRIMARY KEY (task_id, tag_id)
);
CREATE INDEX ix_task_tags_tag ON task_tags(tag_id);
