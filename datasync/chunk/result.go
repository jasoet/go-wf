package chunk

import "cmp"

// Partition is a half-open key range [Start, End) processed as one unit.
type Partition[K cmp.Ordered] struct {
	Start K `json:"start"`
	End   K `json:"end"`
}

// PartitionResult is the per-partition outcome captured in the workflow result.
// Keys are preserved end-to-end so callers can program against them without
// re-parsing strings.
type PartitionResult[K cmp.Ordered] struct {
	Start    K   `json:"start"`
	End      K   `json:"end"`
	Fetched  int `json:"fetched"`
	Inserted int `json:"inserted"`
	Updated  int `json:"updated"`
	Skipped  int `json:"skipped"`
}

// SyncResult is the workflow-level summary aggregating all partitions.
type SyncResult[K cmp.Ordered] struct {
	JobName         string               `json:"jobName"`
	TotalPartitions int                  `json:"totalPartitions"`
	TotalFetched    int                  `json:"totalFetched"`
	TotalInserted   int                  `json:"totalInserted"`
	TotalUpdated    int                  `json:"totalUpdated"`
	TotalSkipped    int                  `json:"totalSkipped"`
	Partitions      []PartitionResult[K] `json:"partitions,omitempty"`
}
