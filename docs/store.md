# Store API

The `workflow/store` package provides a two-layer storage abstraction for persisting workflow data. A low-level **RawStore** handles byte-level I/O, while a generic **Store[T]** adds automatic serialization on top via a pluggable **Codec**.

## Architecture

```
Store[T]  (typed: Save/Load)
   |
TypedStore  ── Codec[T]  (JSONCodec, BytesCodec)
   |
RawStore  (bytes: Upload/Download)
   |
LocalStore / S3Store / InstrumentedStore
```

## RawStore

`RawStore` is the byte-level interface that all storage backends implement:

```go
type RawStore interface {
    Upload(ctx context.Context, key string, data io.Reader) error
    Download(ctx context.Context, key string) (io.ReadCloser, error)
    Delete(ctx context.Context, key string) error
    Exists(ctx context.Context, key string) (bool, error)
    List(ctx context.Context, prefix string) ([]string, error)
    Close() error
}
```

Keys are slash-delimited strings (e.g., `"workflows/run-123/step-a"`). The `Download` caller must close the returned `io.ReadCloser`.

## Store[T]

`Store[T]` is the typed interface that applications typically interact with:

```go
type Store[T any] interface {
    Save(ctx context.Context, key string, value T) error
    Load(ctx context.Context, key string) (T, error)
    Delete(ctx context.Context, key string) error
    Exists(ctx context.Context, key string) (bool, error)
    List(ctx context.Context, prefix string) ([]string, error)
    Close() error
}
```

It mirrors `RawStore` but replaces `Upload`/`Download` with `Save`/`Load`, handling serialization automatically.

## Codec[T]

A `Codec[T]` defines how values are serialized to and from byte streams:

```go
type Codec[T any] interface {
    Encode(value T) (io.Reader, error)
    Decode(reader io.Reader) (T, error)
}
```

Two codecs are provided out of the box:

| Codec | Type | Description |
|---|---|---|
| `JSONCodec[T]` | any | Serializes values as JSON via `encoding/json` |
| `BytesCodec` | `[]byte` | Pass-through, no transformation |

## TypedStore and Convenience Constructors

`TypedStore[T]` bridges `RawStore` and `Codec[T]` to implement `Store[T]`:

```go
// General-purpose: pick any codec
s := store.NewTypedStore[MyStruct](rawStore, &store.JSONCodec[MyStruct]{})

// Convenience: JSON serialization
s := store.NewJSONStore[MyStruct](rawStore)

// Convenience: raw bytes
bs := store.NewBytesStore(rawStore)
```

## KeyBuilder

`KeyBuilder` generates structured, slash-delimited storage keys. It is immutable -- each method returns a new instance, so you can branch safely from a shared base.

```go
kb := store.NewKeyBuilder()

key := kb.
    WithWorkflow("order-process").
    WithRun("run-abc-123").
    WithStep("validate").
    WithName("result.json").
    Build()
// => "order-process/run-abc-123/validate/result.json"

// Branching from a shared base
base := kb.WithWorkflow("etl").WithRun("run-42")
inputKey  := base.WithStep("extract").WithName("data.csv").Build()
outputKey := base.WithStep("load").WithName("output.json").Build()
```

## Implementations

### LocalStore (filesystem)

Stores data as files on the local filesystem. Best suited for development and testing.

```go
raw, err := store.NewLocalStore("/tmp/wf-data")
if err != nil {
    log.Fatal(err)
}
defer raw.Close()
```

Key characteristics:

- The base directory is created automatically if it does not exist.
- Keys are validated against path traversal (`..`, null bytes, backslashes).
- Uploads are capped at **1 GB** (`MaxUploadSize`).
- `Delete` on a missing key is a no-op (no error).
- `Close` is a no-op.

### S3Store (S3-compatible)

Stores data as objects in any S3-compatible service (AWS S3, MinIO, etc.). Recommended for production.

```go
cfg := store.S3Config{
    Endpoint:  "localhost:9000",
    AccessKey: "minioadmin",
    SecretKey: "minioadmin",
    Bucket:    "workflow-data",
    Prefix:    "prod/",    // optional key prefix
    UseSSL:    false,
    Region:    "us-east-1", // defaults to "us-east-1" if empty
}

raw, err := store.NewS3Store(ctx, cfg)
if err != nil {
    log.Fatal(err)
}
defer raw.Close()
```

Key characteristics:

- Uses AWS SDK v2 with static credentials and path-style addressing.
- The bucket is auto-created if it does not exist.
- The optional `Prefix` is prepended to all keys transparently; returned keys from `List` have the prefix stripped.
- `Close` is a no-op.

### InstrumentedStore (OpenTelemetry decorator)

Wraps any `RawStore` with OpenTelemetry tracing spans and metrics. When no OTel config is present in the context, calls delegate directly with zero overhead.

```go
raw, _ := store.NewLocalStore("/tmp/wf-data")
instrumented := store.NewInstrumentedStore(raw)
```

Recorded metrics:

| Metric | Type | Description |
|---|---|---|
| `go_wf.store.operation.total` | Counter | Total operations, labeled by `operation` and `status` |
| `go_wf.store.operation.duration` | Histogram (seconds) | Duration per operation, labeled by `operation` |

Each operation (`Upload`, `Download`, `Delete`, `Exists`, `List`) creates a repository-layer span with the storage key as an attribute.

## Usage Examples

### JSON store with LocalStore

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/jasoet/go-wf/workflow/store"
)

type Report struct {
    ID    string `json:"id"`
    Score int    `json:"score"`
}

func main() {
    ctx := context.Background()

    raw, err := store.NewLocalStore("/tmp/reports")
    if err != nil {
        log.Fatal(err)
    }
    defer raw.Close()

    s := store.NewJSONStore[Report](raw)

    // Save
    err = s.Save(ctx, "reports/2026/march.json", Report{ID: "r-1", Score: 95})
    if err != nil {
        log.Fatal(err)
    }

    // Load
    report, err := s.Load(ctx, "reports/2026/march.json")
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println(report) // {r-1 95}

    // Exists
    ok, _ := s.Exists(ctx, "reports/2026/march.json")
    fmt.Println("exists:", ok) // true

    // List
    keys, _ := s.List(ctx, "reports/2026/")
    fmt.Println("keys:", keys) // [reports/2026/march.json]

    // Delete
    _ = s.Delete(ctx, "reports/2026/march.json")
}
```

### Bytes store with S3

```go
ctx := context.Background()

raw, err := store.NewS3Store(ctx, store.S3Config{
    Endpoint:  "s3.amazonaws.com",
    AccessKey: os.Getenv("AWS_ACCESS_KEY_ID"),
    SecretKey: os.Getenv("AWS_SECRET_ACCESS_KEY"),
    Bucket:    "my-bucket",
    UseSSL:    true,
})
if err != nil {
    log.Fatal(err)
}
defer raw.Close()

bs := store.NewBytesStore(raw)

_ = bs.Save(ctx, "artifacts/output.bin", []byte("binary data"))
data, _ := bs.Load(ctx, "artifacts/output.bin")
fmt.Println(string(data)) // "binary data"
```

### Instrumented S3 store

```go
raw, _ := store.NewS3Store(ctx, cfg)
instrumented := store.NewInstrumentedStore(raw)
s := store.NewJSONStore[MyData](instrumented)

// All Save/Load/Delete/Exists/List calls now emit OTel spans and metrics
_ = s.Save(ctx, "key", MyData{Value: 42})
```

## Summary

| Type | Role |
|---|---|
| `RawStore` | Byte-level storage interface |
| `Store[T]` | Typed storage with automatic serialization |
| `Codec[T]` | Serialization strategy (`JSONCodec`, `BytesCodec`) |
| `TypedStore[T]` | Adapter: RawStore + Codec = Store[T] |
| `KeyBuilder` | Structured key generation |
| `LocalStore` | Filesystem backend (dev/test) |
| `S3Store` | S3-compatible backend (production) |
| `InstrumentedStore` | OTel tracing and metrics decorator |
| `NewJSONStore[T]` | Shorthand for `NewTypedStore` with `JSONCodec` |
| `NewBytesStore` | Shorthand for `NewTypedStore` with `BytesCodec` |
