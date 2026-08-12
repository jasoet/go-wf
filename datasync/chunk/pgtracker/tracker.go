// Package pgtracker provides Postgres-backed implementations of the chunk
// package's progress trackers.
//
// chunk defines ProgressTracker/TimeProgressTracker but deliberately ships no
// implementation, because persistence is the application's concern. In
// practice almost every chunked sync needs the same thing: a small table
// holding one cursor per job. This package is that table.
//
// It depends only on database/sql — the caller supplies the driver and the
// pool, so pulling this in adds no driver dependency to go-wf.
package pgtracker

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"time"
)

// DefaultTable is the table name used when none is configured.
const DefaultTable = "sync_watermark"

// safeIdentifier guards the table name, which is interpolated into DDL and DML
// because SQL placeholders cannot parameterise identifiers. Restricting it to
// simple lowercase identifiers (optionally schema-qualified) keeps that safe.
var safeIdentifier = regexp.MustCompile(`^[a-z_][a-z0-9_]*(\.[a-z_][a-z0-9_]*)?$`)

// TimeTracker persists a time.Time cursor per job in Postgres. It satisfies
// chunk.TimeProgressTracker.
type TimeTracker struct {
	db    *sql.DB
	table string
}

// Option configures a TimeTracker.
type Option func(*TimeTracker)

// WithTable overrides the table name (default: sync_watermark). The name may be
// a bare identifier or schema-qualified, lowercase, and must already be a valid
// SQL identifier — it is interpolated, not parameterised.
func WithTable(name string) Option {
	return func(t *TimeTracker) { t.table = name }
}

// NewTimeTracker returns a tracker over db. Call EnsureSchema once at startup,
// or create the table yourself with the DDL from SchemaDDL.
func NewTimeTracker(db *sql.DB, opts ...Option) (*TimeTracker, error) {
	if db == nil {
		return nil, errors.New("pgtracker: db is required")
	}

	t := &TimeTracker{db: db, table: DefaultTable}
	for _, opt := range opts {
		opt(t)
	}

	if !safeIdentifier.MatchString(t.table) {
		return nil, fmt.Errorf("pgtracker: unsafe table name %q", t.table)
	}
	return t, nil
}

// SchemaDDL returns the CREATE TABLE statement for this tracker's table, for
// applications that manage their own migrations.
func (t *TimeTracker) SchemaDDL() string {
	return fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
	job_name   TEXT PRIMARY KEY,
	cursor_at  TIMESTAMPTZ NOT NULL,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
)`, t.table)
}

// EnsureSchema creates the tracker table if it does not exist. Safe to call on
// every startup.
func (t *TimeTracker) EnsureSchema(ctx context.Context) error {
	if _, err := t.db.ExecContext(ctx, t.SchemaDDL()); err != nil {
		return fmt.Errorf("pgtracker: ensure schema: %w", err)
	}
	return nil
}

// Cursor returns the last completed partition end for jobName. The boolean
// reports whether a cursor has ever been recorded — distinguishing "never ran"
// from "ran, and the cursor happens to be the zero time".
func (t *TimeTracker) Cursor(ctx context.Context, jobName string) (time.Time, bool, error) {
	//nolint:gosec // G201: the table name is interpolated because SQL cannot
	// parameterise identifiers; it is validated against safeIdentifier at
	// construction. The job name is a real parameter.
	query := fmt.Sprintf(`SELECT cursor_at FROM %s WHERE job_name = $1`, t.table)

	var cursor time.Time
	err := t.db.QueryRowContext(ctx, query, jobName).Scan(&cursor)

	switch {
	case errors.Is(err, sql.ErrNoRows):
		return time.Time{}, false, nil
	case err != nil:
		return time.Time{}, false, fmt.Errorf("pgtracker: read cursor for %q: %w", jobName, err)
	}
	return cursor.UTC(), true, nil
}

// Advance records that every partition ending at or before completed has been
// processed.
//
// The cursor only ever moves forward: a retried or out-of-order workflow may
// call Advance with an older value, and rewinding would silently re-process —
// or worse, skip — partitions. GREATEST makes the call idempotent and
// order-independent.
func (t *TimeTracker) Advance(ctx context.Context, jobName string, completed time.Time) error {
	//nolint:gosec // G201: identifier interpolation, validated at construction.
	query := fmt.Sprintf(`
		INSERT INTO %s (job_name, cursor_at, updated_at)
		VALUES ($1, $2, now())
		ON CONFLICT (job_name) DO UPDATE
		SET cursor_at  = GREATEST(%s.cursor_at, EXCLUDED.cursor_at),
		    updated_at = now()`, t.table, t.table)

	if _, err := t.db.ExecContext(ctx, query, jobName, completed.UTC()); err != nil {
		return fmt.Errorf("pgtracker: advance cursor for %q: %w", jobName, err)
	}
	return nil
}

// Reset removes the cursor for jobName, so the next run starts from the
// configured lookback window again. Intended for operational recovery and
// tests, not normal operation.
func (t *TimeTracker) Reset(ctx context.Context, jobName string) error {
	//nolint:gosec // G201: identifier interpolation, validated at construction.
	query := fmt.Sprintf(`DELETE FROM %s WHERE job_name = $1`, t.table)

	if _, err := t.db.ExecContext(ctx, query, jobName); err != nil {
		return fmt.Errorf("pgtracker: reset cursor for %q: %w", jobName, err)
	}
	return nil
}
