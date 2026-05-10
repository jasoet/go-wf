package chunk

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestKey_RoundTrip_UTC(t *testing.T) {
	in := time.Date(2026, 5, 10, 23, 30, 0, 0, time.UTC)
	k := TimeToKey(in)
	out := KeyToTime(k)
	assert.True(t, in.Equal(out), "round-trip preserves instant: in=%s out=%s", in, out)
	assert.Equal(t, time.UTC, out.Location(), "KeyToTime returns UTC")
}

func TestKey_RoundTrip_NonUTC(t *testing.T) {
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		t.Skipf("Asia/Jakarta tz unavailable: %v", err)
	}
	in := time.Date(2026, 5, 10, 23, 30, 0, 0, loc)
	k := TimeToKey(in)
	out := KeyToTime(k)
	assert.True(t, in.Equal(out), "round-trip preserves instant across zones: in=%s out=%s", in, out)
}

func TestKey_Monotonicity(t *testing.T) {
	a := time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC)
	b := a.Add(1 * time.Second)
	assert.Less(t, TimeToKey(a), TimeToKey(b), "monotonicity preserved across seconds")
}

func TestKey_DSTBoundary(t *testing.T) {
	// US/Eastern springs forward 2026-03-08 02:00 -> 03:00. Two distinct
	// instants either side must remain ordered when converted to keys.
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Skipf("America/New_York tz unavailable: %v", err)
	}
	before := time.Date(2026, 3, 8, 1, 30, 0, 0, loc)
	after := time.Date(2026, 3, 8, 3, 30, 0, 0, loc)
	assert.Less(t, TimeToKey(before), TimeToKey(after))
}
