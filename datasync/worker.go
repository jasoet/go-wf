package datasync

// This file provides convenience documentation.
// Worker registration is in datasync/workflow package:
//   - workflow.RegisterJob[T, U](w, job) — register a single job
//   - workflow.BuildJobRegistration[T, U](job, disabled) — type-erased registration
//   - workflow.FullJobRegistration — type-erased job metadata for worker managers
