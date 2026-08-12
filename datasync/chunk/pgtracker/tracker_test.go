package pgtracker

import (
	"database/sql"
	"testing"
)

// stubDB returns a non-nil *sql.DB for constructor tests. These tests never
// execute a query, so an unconnected handle is sufficient and keeps them fast
// and driver-free; behavior against a real server is covered by the
// integration tests.
func stubDB(t *testing.T) *sql.DB {
	t.Helper()
	return &sql.DB{}
}

func TestNewTimeTracker_RequiresDB(t *testing.T) {
	if _, err := NewTimeTracker(nil); err == nil {
		t.Fatal("expected an error when db is nil")
	}
}

func TestNewTimeTracker_DefaultTable(t *testing.T) {
	tracker, err := NewTimeTracker(stubDB(t))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if tracker.table != DefaultTable {
		t.Fatalf("table = %q, want %q", tracker.table, DefaultTable)
	}
}

func TestNewTimeTracker_RejectsUnsafeTableNames(t *testing.T) {
	unsafe := []string{
		"sync watermark",
		"sync;DROP TABLE users",
		"Sync_Watermark",
		"sync-watermark",
		"",
		"a.b.c",
	}

	for _, name := range unsafe {
		if _, err := NewTimeTracker(stubDB(t), WithTable(name)); err == nil {
			t.Errorf("table name %q was accepted; identifiers are interpolated, not parameterised", name)
		}
	}
}

func TestNewTimeTracker_AcceptsSchemaQualifiedTable(t *testing.T) {
	tracker, err := NewTimeTracker(stubDB(t), WithTable("sync.watermark"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if tracker.table != "sync.watermark" {
		t.Fatalf("table = %q, want %q", tracker.table, "sync.watermark")
	}
}

func TestSchemaDDL_UsesConfiguredTable(t *testing.T) {
	tracker, err := NewTimeTracker(stubDB(t), WithTable("custom_cursor"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ddl := tracker.SchemaDDL()
	if want := "CREATE TABLE IF NOT EXISTS custom_cursor"; !contains(ddl, want) {
		t.Fatalf("DDL %q does not contain %q", ddl, want)
	}
}

func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && (haystack == needle || indexOf(haystack, needle) >= 0)
}

func indexOf(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}
