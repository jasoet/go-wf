package datasync

import "context"

// WriteResult contains statistics from a sink write operation.
type WriteResult struct {
	Inserted int `json:"inserted"`
	Updated  int `json:"updated"`
	Skipped  int `json:"skipped"`
}

// Add merges another WriteResult into this one.
func (r *WriteResult) Add(other WriteResult) {
	r.Inserted += other.Inserted
	r.Updated += other.Updated
	r.Skipped += other.Skipped
}

// Total returns the total number of records processed.
func (r *WriteResult) Total() int {
	return r.Inserted + r.Updated + r.Skipped
}

// Sink consumes data of type U and writes it to a destination system.
type Sink[U any] interface {
	// Name returns a unique identifier for this sink.
	Name() string
	// Write persists a batch of records and returns write statistics.
	Write(ctx context.Context, records []U) (WriteResult, error)
}
