package datasync

// This file provides convenience documentation.
// Worker registration is in datasync/workflow package:
//   - workflow.RegisterJob[T, U](w, job) — register a single job
//
// The datasync/builder package provides SyncJobBuilder which returns a
// *job.Definition (from github.com/jasoet/pkg/v2/temporal/job) via Build().
