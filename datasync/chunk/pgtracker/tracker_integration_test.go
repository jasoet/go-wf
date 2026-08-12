//go:build integration

package pgtracker_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/jasoet/go-wf/datasync/chunk"
	"github.com/jasoet/go-wf/datasync/chunk/pgtracker"
)

func newTracker(t *testing.T) *pgtracker.TimeTracker {
	t.Helper()
	ctx := context.Background()

	container, err := tcpostgres.Run(ctx, "postgres:18-alpine",
		tcpostgres.WithDatabase("tracker"),
		tcpostgres.WithUsername("test"),
		tcpostgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(2*time.Minute),
		),
	)
	if err != nil {
		t.Fatalf("start postgres: %v", err)
	}
	t.Cleanup(func() { _ = testcontainers.TerminateContainer(container) })

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("connection string: %v", err)
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	tracker, err := pgtracker.NewTimeTracker(db)
	if err != nil {
		t.Fatalf("new tracker: %v", err)
	}
	if err := tracker.EnsureSchema(ctx); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}
	return tracker
}

// The tracker must satisfy the interface it exists to implement.
var _ chunk.TimeProgressTracker = (*pgtracker.TimeTracker)(nil)

func TestTimeTracker_NoCursorReportsAbsent(t *testing.T) {
	tracker := newTracker(t)

	_, exists, err := tracker.Cursor(context.Background(), "orders-sync")
	if err != nil {
		t.Fatalf("cursor: %v", err)
	}
	if exists {
		t.Fatal("a job that has never run must report no cursor")
	}
}

func TestTimeTracker_AdvanceThenCursor(t *testing.T) {
	tracker := newTracker(t)
	ctx := context.Background()
	want := time.Date(2026, 8, 12, 10, 0, 0, 0, time.UTC)

	if err := tracker.Advance(ctx, "orders-sync", want); err != nil {
		t.Fatalf("advance: %v", err)
	}

	got, exists, err := tracker.Cursor(ctx, "orders-sync")
	if err != nil {
		t.Fatalf("cursor: %v", err)
	}
	if !exists {
		t.Fatal("cursor must exist after advance")
	}
	if !got.Equal(want) {
		t.Fatalf("cursor = %s, want %s", got, want)
	}
}

func TestTimeTracker_AdvanceIsIdempotent(t *testing.T) {
	tracker := newTracker(t)
	ctx := context.Background()
	at := time.Date(2026, 8, 12, 10, 0, 0, 0, time.UTC)

	for range 3 {
		if err := tracker.Advance(ctx, "orders-sync", at); err != nil {
			t.Fatalf("advance: %v", err)
		}
	}

	got, _, err := tracker.Cursor(ctx, "orders-sync")
	if err != nil {
		t.Fatalf("cursor: %v", err)
	}
	if !got.Equal(at) {
		t.Fatalf("cursor = %s, want %s", got, at)
	}
}

func TestTimeTracker_CursorNeverRewinds(t *testing.T) {
	tracker := newTracker(t)
	ctx := context.Background()
	later := time.Date(2026, 8, 12, 12, 0, 0, 0, time.UTC)
	earlier := time.Date(2026, 8, 12, 6, 0, 0, 0, time.UTC)

	if err := tracker.Advance(ctx, "orders-sync", later); err != nil {
		t.Fatalf("advance later: %v", err)
	}
	// A retried or out-of-order workflow may report an older completion.
	// Rewinding would silently re-process or skip partitions.
	if err := tracker.Advance(ctx, "orders-sync", earlier); err != nil {
		t.Fatalf("advance earlier: %v", err)
	}

	got, _, err := tracker.Cursor(ctx, "orders-sync")
	if err != nil {
		t.Fatalf("cursor: %v", err)
	}
	if !got.Equal(later) {
		t.Fatalf("cursor = %s, want %s (must not rewind)", got, later)
	}
}

func TestTimeTracker_JobsAreIndependent(t *testing.T) {
	tracker := newTracker(t)
	ctx := context.Background()
	ordersAt := time.Date(2026, 8, 12, 10, 0, 0, 0, time.UTC)
	usersAt := time.Date(2026, 8, 11, 10, 0, 0, 0, time.UTC)

	if err := tracker.Advance(ctx, "orders-sync", ordersAt); err != nil {
		t.Fatalf("advance orders: %v", err)
	}
	if err := tracker.Advance(ctx, "users-sync", usersAt); err != nil {
		t.Fatalf("advance users: %v", err)
	}

	orders, _, err := tracker.Cursor(ctx, "orders-sync")
	if err != nil {
		t.Fatalf("cursor orders: %v", err)
	}
	users, _, err := tracker.Cursor(ctx, "users-sync")
	if err != nil {
		t.Fatalf("cursor users: %v", err)
	}

	if !orders.Equal(ordersAt) || !users.Equal(usersAt) {
		t.Fatalf("cursors bled between jobs: orders=%s users=%s", orders, users)
	}
}

func TestTimeTracker_Reset(t *testing.T) {
	tracker := newTracker(t)
	ctx := context.Background()

	if err := tracker.Advance(ctx, "orders-sync", time.Now().UTC()); err != nil {
		t.Fatalf("advance: %v", err)
	}
	if err := tracker.Reset(ctx, "orders-sync"); err != nil {
		t.Fatalf("reset: %v", err)
	}

	_, exists, err := tracker.Cursor(ctx, "orders-sync")
	if err != nil {
		t.Fatalf("cursor: %v", err)
	}
	if exists {
		t.Fatal("cursor must be absent after reset")
	}
}

func TestTimeTracker_EnsureSchemaIsIdempotent(t *testing.T) {
	tracker := newTracker(t)

	if err := tracker.EnsureSchema(context.Background()); err != nil {
		t.Fatalf("second EnsureSchema: %v", err)
	}
}
