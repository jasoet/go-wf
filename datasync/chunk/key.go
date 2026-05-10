package chunk

import "time"

// TimeToKey converts a time.Time to its int64 Unix-nanosecond representation.
// Used internally by DateChunkedSync to project time.Time partition keys onto
// the cmp.Ordered constraint required by ChunkedSync.
func TimeToKey(t time.Time) int64 {
	return t.UnixNano()
}

// KeyToTime converts a Unix-nanosecond key back to a UTC time.Time. Callers
// that need a different display zone should call .In(loc) on the result.
func KeyToTime(k int64) time.Time {
	return time.Unix(0, k).UTC()
}
