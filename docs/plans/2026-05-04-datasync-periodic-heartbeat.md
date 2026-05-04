# datasync: Periodic Heartbeat During SyncData Activity

**Filed:** 2026-05-04
**Reporter:** caller from `gitlab.com/paramartha/product/para-sync` (consumer of `github.com/jasoet/go-wf/datasync`)
**Status:** Open — workaround applied downstream, library fix recommended
**Affected package:** `datasync/activity`

## Summary

`datasync/activity/sync.go::SyncData` only calls `activity.RecordHeartbeat` at two checkpoints — after `source.Fetch` returns (line 84) and after `sink.Write` returns (line 134). It does NOT heartbeat *during* either phase. When a `Source.Fetch` (or `Sink.Write`) call takes longer than the configured `HeartbeatTimeout`, Temporal kills the activity even though it is making forward progress.

This is a latent bug in the library: any consumer with a slow source DB / API will see intermittent activity heartbeat-timeout failures, regardless of how the consumer configures `HeartbeatTimeout` (any value < the actual fetch duration trips it).

## Affected Code

`datasync/activity/sync.go`:

```go
// === Fetch ===
fetchStart := time.Now()
fetchLC := pkgotel.Layers.StartService(lc.Context(), "datasync", "Fetch", ...)
records, err := a.source.Fetch(fetchLC.Context())   // <-- can run for minutes; no heartbeat during
// ...
activity.RecordHeartbeat(ctx, fmt.Sprintf("fetched %d records", len(records)))   // line 84 — fires AFTER fetch returns
```

The default `defaultHeartbeatTimeout = 30 * time.Second` (`datasync/workflow/sync.go:18`) means any `Fetch` taking >30s reliably fails.

## Reproduction (observed in production)

Para-sync staging, 2026-05-04 01:30 UTC, namespace `para-sync-ticketing`:

```
ID  Time                  Type
 5  2026-05-04T01:30:00Z  ActivityTaskScheduled
 6  2026-05-04T01:32:34Z  ActivityTaskStarted        (2.5 min wait — worker queue saturated)
 7  2026-05-04T01:33:05Z  ActivityTaskTimedOut       (31s after start — 30s heartbeat tripped)
 8  2026-05-04T01:33:05Z  WorkflowExecutionFailed

Failure: activity Heartbeat timeout
```

Same pattern reproduced in para-sync namespace `para-sync-reference` (`goers-expand-ticket-sync`) earlier the same day — different consumer job, identical failure shape.

## Impact in para-sync (current consumer)

Three jobs configured at the most aggressive `HeartbeatTimeout: 30s` are intermittently failing (~1 in 10 runs):

| Consumer job | File:line in para-sync |
|---|---|
| `goers-expand-ticket-sync` | `internal/reference/job/expand_ticket.go:27` |
| `ticketing-master-sync` | `internal/ticketing/register.go:95` |
| `erp-coa-sync` | `internal/erp/register.go:66` (latent — same risk, not yet observed) |

A larger set of jobs is configured at `HeartbeatTimeout: 1m` — same library bug, just more tolerant. They will fail under any source slowdown >1m.

Para-sync mitigates by re-running on the next short-cadence schedule (full-replace or rolling-window sync, no data loss). The bug is purely operational noise — but it pollutes failure dashboards and forces consumers to set `HeartbeatTimeout` defensively high (defeats the purpose of heartbeats).

## Recommended Fix

Spawn a periodic heartbeat goroutine for the *lifetime* of `SyncData` so the activity heartbeats independently of whether `Fetch`/`Map`/`Write` happens to be in flight.

**Reference implementation** — para-sync's `internal/syncwf` package solved the identical problem in commit `1d40a41`:

```go
// At the top of SyncData, before Fetch:
hbDone := make(chan struct{})
defer close(hbDone)
go func() {
    ticker := time.NewTicker(10 * time.Second)
    defer ticker.Stop()
    for {
        select {
        case <-hbDone:
            return
        case <-ctx.Done():
            return
        case <-ticker.C:
            activity.RecordHeartbeat(ctx, fmt.Sprintf("syncing %s (working)", input.JobName))
        }
    }
}()
```

The 10s tick is well under the typical 30s/1m `HeartbeatTimeout` defaults, so the activity stays healthy as long as it isn't deadlocked.

Suggested companion changes:
1. Bump `defaultHeartbeatTimeout` from 30s → something less aggressive (para-sync's syncwf bumped to 5m as a safety net — the goroutine is the primary mechanism, the timeout is the backstop).
2. Add a unit test that verifies the goroutine actually fires during a slow `Source.Fetch` (mock fetcher that sleeps; assert `RecordHeartbeat` invocation count > 1). Para-sync's equivalent test: `internal/syncwf/chunked_test.go::TestRunChunk_HeartbeatsDuringSlowFetcher`.

## Out of Scope (do not bundle)

- Changing the heartbeat *payload* format (existing `"fetched N records"` and `"wrote N records"` checkpoints are valuable for debugging — keep them, the goroutine adds a third "working" beat in between).
- Touching `builder.go` or `workflow/sync.go` configuration plumbing — the fix is local to `activity/sync.go`.

## Verification After Fix

Consumer side: `HeartbeatTimeout: 30s` with a `Source.Fetch` that sleeps 2 minutes should now succeed (not time out). Para-sync can roll back its defensive `HeartbeatTimeout` bumps once a fixed `go-wf` version ships.
