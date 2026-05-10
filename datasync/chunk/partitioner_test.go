package chunk

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDatePartitioner_BasicWindow(t *testing.T) {
	loc := time.UTC
	now := time.Date(2026, 5, 10, 0, 0, 0, 0, loc)
	p := &DatePartitioner{
		Now:       func() time.Time { return now },
		Loc:       loc,
		LookBack:  72 * time.Hour,
		ChunkSize: 24 * time.Hour,
	}

	parts, err := p.Partitions(context.Background())
	require.NoError(t, err)
	require.Len(t, parts, 3)

	assert.Equal(t, TimeToKey(time.Date(2026, 5, 7, 0, 0, 0, 0, loc)), parts[0].Start)
	assert.Equal(t, TimeToKey(time.Date(2026, 5, 8, 0, 0, 0, 0, loc)), parts[0].End)
	assert.Equal(t, TimeToKey(time.Date(2026, 5, 9, 0, 0, 0, 0, loc)), parts[1].End)
	assert.Equal(t, TimeToKey(time.Date(2026, 5, 10, 0, 0, 0, 0, loc)), parts[2].End)
}

func TestDatePartitioner_LastChunkClampedToNow(t *testing.T) {
	loc := time.UTC
	now := time.Date(2026, 5, 10, 6, 0, 0, 0, loc)
	p := &DatePartitioner{
		Now:       func() time.Time { return now },
		Loc:       loc,
		LookBack:  48 * time.Hour,
		ChunkSize: 24 * time.Hour,
	}
	parts, err := p.Partitions(context.Background())
	require.NoError(t, err)
	require.Len(t, parts, 3)
	// Last partition end clamped to now (06:00), not the next midnight.
	// 48h LookBack from now=05-10 06:00 aligns start to 05-08 00:00.
	// Iter: [05-08, 05-09), [05-09, 05-10), [05-10, 05-10 06:00 clamped to now).
	assert.Equal(t, TimeToKey(now), parts[len(parts)-1].End)
}

func TestDatePartitioner_AlignsToCalendarMidnightInTimezone(t *testing.T) {
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		t.Skipf("tz unavailable: %v", err)
	}
	// Now is mid-day in WIB; alignment should push start to a WIB midnight.
	now := time.Date(2026, 5, 10, 14, 30, 0, 0, loc)
	p := &DatePartitioner{
		Now:       func() time.Time { return now },
		Loc:       loc,
		LookBack:  24 * time.Hour,
		ChunkSize: 24 * time.Hour,
	}
	parts, err := p.Partitions(context.Background())
	require.NoError(t, err)
	// Simple algorithm: now=14:30 WIB, start aligns to 05-09 00:00 WIB.
	// Iter: [05-09 00:00, 05-10 00:00), [05-10 00:00, 05-10 14:30 clamped to now).
	require.Len(t, parts, 2)
	startTime := KeyToTime(parts[0].Start).In(loc)
	assert.Equal(t, 0, startTime.Hour())
	assert.Equal(t, 0, startTime.Minute())
}

func TestDatePartitioner_Validation(t *testing.T) {
	t.Run("LookBack required", func(t *testing.T) {
		p := &DatePartitioner{Now: time.Now, Loc: time.UTC, ChunkSize: time.Hour}
		_, err := p.Partitions(context.Background())
		assert.Error(t, err)
	})
	t.Run("ChunkSize required", func(t *testing.T) {
		p := &DatePartitioner{Now: time.Now, Loc: time.UTC, LookBack: time.Hour}
		_, err := p.Partitions(context.Background())
		assert.Error(t, err)
	})
}

func TestDatePartitioner_ZeroNowDefaultsToTimeNow(t *testing.T) {
	// When Now is nil, partitioner should fall back to time.Now and produce
	// a non-empty partition list for a non-zero LookBack.
	p := &DatePartitioner{
		Loc:       time.UTC,
		LookBack:  time.Hour,
		ChunkSize: time.Hour,
	}
	parts, err := p.Partitions(context.Background())
	require.NoError(t, err)
	assert.NotEmpty(t, parts)
}
