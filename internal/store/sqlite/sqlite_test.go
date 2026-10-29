package sqlite

import (
	"context"
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
