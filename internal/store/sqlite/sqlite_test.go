package sqlite

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/mahm0udnasr/tasker/internal/store"
	"github.com/mahm0udnasr/tasker/internal/store/storetest"
)

func open(t *testing.T, path string) *Store {
	t.Helper()
	s, err := Open(context.Background(), path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	return s
}

func TestConformance(t *testing.T) {
	storetest.Run(t, func(t *testing.T) store.Store {
		return open(t, filepath.Join(t.TempDir(), "tasker.db"))
	})
}

func TestMigrationsAreIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "dir", "tasker.db")
	s := open(t, path)
	v1, err := s.SchemaVersion(context.Background())
	if err != nil || v1 < 1 {
		t.Fatalf("version = %d, %v", v1, err)
	}
	_ = s.Close()

	s = open(t, path) // re-open must not re-apply
	defer s.Close()
	v2, _ := s.SchemaVersion(context.Background())
	if v1 != v2 {
		t.Errorf("version changed on reopen: %d → %d", v1, v2)
	}
}

func TestRejectsNewerSchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tasker.db")
	s := open(t, path)
	if _, err := s.db.Exec(`INSERT INTO schema_migrations(version, applied_at) VALUES (9999, 0)`); err != nil {
		t.Fatal(err)
	}
	_ = s.Close()
	if _, err := Open(context.Background(), path); err == nil {
		t.Fatal("expected error opening a newer schema")
	}
}

// TestUpgradeFromV1 simulates a database created by an older tasker that
// only had migration 0001, with data in it.
func TestUpgradeFromV1(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "old.db")
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatal(err)
	}
	v1, err := migrationFS.ReadFile("migrations/0001_init.sql")
	if err != nil {
		t.Fatal(err)
	}
	for _, stmt := range []string{
		`CREATE TABLE schema_migrations (version INTEGER PRIMARY KEY, applied_at INTEGER NOT NULL)`,
		string(v1),
		`INSERT INTO schema_migrations VALUES (1, 0)`,
		`INSERT INTO tasks (id, title, created_at, updated_at) VALUES ('11111111-0000-4000-8000-000000000000', 'old task', 0, 0)`,
	} {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			t.Fatal(err)
		}
	}
	_ = db.Close()

	s := open(t, path)
	defer s.Close()
	if v, _ := s.SchemaVersion(ctx); v != 2 {
		t.Errorf("schema version = %d, want 2", v)
	}
	task, err := s.GetTask(ctx, "11111111")
	if err != nil || task.Title != "old task" || task.Estimate != 0 {
		t.Fatalf("old data after upgrade: %+v, %v", task, err)
	}
}
