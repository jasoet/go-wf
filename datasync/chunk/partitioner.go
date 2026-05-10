package chunk

import (
	"cmp"
	"context"
	"errors"
	"time"
)

// Partitioner generates a sequence of partition keys for a sync run.
//
// Implementations must be deterministic for a given moment in time — the
// result is recorded in workflow history and used as the basis for activity
// scheduling. A non-deterministic partitioner would still work (its output is
// captured in history), but consumers should prefer deterministic ones for
// debuggability.
type Partitioner[K cmp.Ordered] interface {
	Partitions(ctx context.Context) ([]Partition[K], error)
}

// DatePartitioner is a time-range partitioner that emits int64 Unix-nano keys
// covering [now-LookBack, now) in ChunkSize steps. Start of the range is
// aligned to the most recent midnight in Loc so a 24h ChunkSize yields a
// single calendar day per partition.
//
// Used internally by DateChunkedSync; library consumers normally don't
// instantiate DatePartitioner directly.
type DatePartitioner struct {
	// Now returns the reference instant. If nil, time.Now is used.
	Now func() time.Time
	// Loc is the timezone used for midnight alignment. Defaults to UTC if nil.
	Loc *time.Location
	// LookBack is the total window size. Must be > 0.
	LookBack time.Duration
	// ChunkSize is the size of each partition. Must be > 0.
	ChunkSize time.Duration
}

// Partitions implements Partitioner[int64].
//
// The partitioning proceeds as follows:
//
//  1. alignedStart = midnight(now − LookBack) in Loc, aligning the window start
//     to a calendar-day boundary so a 24 h ChunkSize yields whole calendar days.
//  2. naturalEnd = alignedStart + LookBack — the canonical window end after
//     alignment.
//  3. The loop emits full ChunkSize partitions in [alignedStart, naturalEnd).
//  4. If now > naturalEnd and the trailing gap (now − naturalEnd) is less than
//     half a ChunkSize, the last partition's End is extended to now so the
//     short partial period is included rather than silently dropped.
func (d *DatePartitioner) Partitions(_ context.Context) ([]Partition[int64], error) {
	if d.LookBack <= 0 {
		return nil, errors.New("DatePartitioner: LookBack must be > 0")
	}
	if d.ChunkSize <= 0 {
		return nil, errors.New("DatePartitioner: ChunkSize must be > 0")
	}
	loc := d.Loc
	if loc == nil {
		loc = time.UTC
	}
	nowFn := d.Now
	if nowFn == nil {
		nowFn = time.Now
	}
	now := nowFn()
	alignedStart := alignToMidnight(now.Add(-d.LookBack), loc)
	naturalEnd := alignedStart.Add(d.LookBack)

	var out []Partition[int64]
	for cur := alignedStart; cur.Before(naturalEnd); cur = cur.Add(d.ChunkSize) {
		end := cur.Add(d.ChunkSize)
		if end.After(naturalEnd) {
			end = naturalEnd
		}
		out = append(out, Partition[int64]{
			Start: TimeToKey(cur),
			End:   TimeToKey(end),
		})
	}

	// If now falls slightly past naturalEnd (trailing gap < half a ChunkSize),
	// extend the last partition's End to now to capture the short partial period.
	if now.After(naturalEnd) && now.Sub(naturalEnd) < d.ChunkSize/2 && len(out) > 0 {
		out[len(out)-1].End = TimeToKey(now)
	}

	return out, nil
}

func alignToMidnight(t time.Time, loc *time.Location) time.Time {
	w := t.In(loc)
	return time.Date(w.Year(), w.Month(), w.Day(), 0, 0, 0, 0, loc)
}
