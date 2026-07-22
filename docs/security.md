# Security Model

This document describes the trust boundaries of go-wf and how to handle secrets safely.

## Trust Boundary: Task Queue = Code Execution

**Anyone who can submit a workflow to a go-wf task queue can execute arbitrary code on the
worker host.** This is by design:

- Container workflows run arbitrary images with arbitrary commands on the worker's
  Docker/Podman daemon.
- Function workflows dispatch to any registered handler with caller-controlled args.
- The volume denylist (`container/payload` `ValidateVolumes` — blocks `/etc`, `/root`,
  `/proc`, `/sys`, `/dev`, container runtime sockets) is defense-in-depth against
  accidental self-harm, **not a sandbox**. It does not stop a deliberately malicious
  workflow input.

Deploy accordingly: treat Temporal namespace/task-queue access as equivalent to host access
on the workers. Do not expose submission to untrusted callers.

## Workflow Inputs Persist in Temporal History

Every workflow input and activity input is recorded in Temporal workflow history,
**visible in the Temporal UI and retained for the namespace's retention period**. Never
place plaintext secrets in:

- `ContainerExecutionInput.Env` / `Command` / `Entrypoint`
- HTTP template headers (inlined into `Command`) or script template env
- `FunctionExecutionInput.Args` / `Data` / `Env`
- `SyncExecutionInput.Metadata` / activity `Params`
- Schedule inputs (they are re-submitted verbatim)

## Secret References (`secret://`)

Payload env values may carry a **reference** instead of a plaintext value. Activities
resolve references worker-side at runtime, so only the reference — never the value —
enters workflow history:

```go
Env: map[string]string{
    "POSTGRES_PASSWORD": "secret://PGPASS",   // resolved on the worker
    "POSTGRES_DB":       "app",                // plain values pass through
}
```

The default resolver reads worker environment variables with the `SECRET_` prefix:
`secret://PGPASS` resolves from `SECRET_PGPASS`. Resolution is supported today in
`StartContainerActivity` and `ExecuteFunctionActivity` (their `Env` maps).

To use a different backend (files, Vault, cloud secret managers), replace the resolver at
worker startup:

```go
import "github.com/jasoet/go-wf/workflow/secrets"

secrets.SetDefault(secrets.ResolverFunc(func(ctx context.Context, ref string) (string, error) {
    return myVault.Read(ctx, ref)
}))
```

A missing reference fails the activity (infrastructure error → Temporal retry), it never
silently injects an empty value.

## Already-Safe Patterns

- **datasync** resolves sources and sinks worker-side by *name* (`SourceName`/`SinkName`);
  credentials live in worker-side Go objects and never cross the workflow boundary.
- **S3 configs** (`store.S3Config`) are constructed at worker startup (e.g. from env vars)
  and are never serialized into workflow inputs.
