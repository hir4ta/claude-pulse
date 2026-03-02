package store

import (
	"path/filepath"
	"testing"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	st, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

func TestOpenAndMigrate(t *testing.T) {
	t.Parallel()
	st := openTestStore(t)

	// Verify schema version.
	var uv int
	err := st.DB().QueryRow("PRAGMA user_version").Scan(&uv)
	if err != nil {
		t.Fatal(err)
	}
	if uv != SchemaVersion() {
		t.Errorf("user_version = %d, want %d", uv, SchemaVersion())
	}
}

func TestDefaultDBPath(t *testing.T) {
	t.Parallel()
	p := DefaultDBPath()
	if p == "" {
		t.Fatal("DefaultDBPath returned empty string")
	}
	if filepath.Base(p) != "pulse.db" {
		t.Errorf("DefaultDBPath base = %q, want pulse.db", filepath.Base(p))
	}
}
