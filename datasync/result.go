package datasync

import "time"

// Result contains the outcome of a sync run.
type Result struct {
	TotalFetched   int           `json:"totalFetched"`
	WriteResult    WriteResult   `json:"writeResult"`
	ProcessingTime time.Duration `json:"processingTime"`
}
