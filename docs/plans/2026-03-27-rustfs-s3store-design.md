# Design: Replace MinIO with RustFS + AWS SDK v2

**Date:** 2026-03-27
**Status:** Approved
**Type:** Library swap + rename

## Motivation

Replace the MinIO-specific client library (`minio-go`) with the standard AWS SDK for Go v2, and swap the MinIO container for RustFS in local dev. This makes the S3-compatible store truly generic — it works with any S3-compatible backend (RustFS, MinIO, AWS S3, etc.).

## Scope

### 1. Rename types and file

| From | To |
|------|----|
| `minio.go` | `s3.go` |
| `minio_integration_test.go` | `s3_integration_test.go` |
| `MinioStore` | `S3Store` |
| `MinioConfig` | `S3Config` |
| `NewMinioStore()` | `NewS3Store()` |
| `testMinioConfig` | `testS3Config` |
| `TestMinioStore_*` | `TestS3Store_*` |

### 2. Replace client library

| minio-go | AWS SDK v2 |
|----------|-----------|
| `minio.Client` | `s3.Client` |
| `PutObject(ctx, bucket, key, reader, size, opts)` | `PutObject(ctx, &s3.PutObjectInput{...})` |
| `GetObject(ctx, bucket, key, opts)` | `GetObject(ctx, &s3.GetObjectInput{...})` |
| `RemoveObject(ctx, bucket, key, opts)` | `DeleteObject(ctx, &s3.DeleteObjectInput{...})` |
| `StatObject(ctx, bucket, key, opts)` | `HeadObject(ctx, &s3.HeadObjectInput{...})` |
| `ListObjects(ctx, bucket, opts)` (channel) | `ListObjectsV2(ctx, &s3.ListObjectsV2Input{...})` (paginated) |
| `BucketExists(ctx, bucket)` | `HeadBucket(ctx, &s3.HeadBucketInput{...})` |
| `MakeBucket(ctx, bucket, opts)` | `CreateBucket(ctx, &s3.CreateBucketInput{...})` |
| Error check: `minio.ToErrorResponse(err).Code == "NoSuchKey"` | Error check: `smithy.APIError` with code `NoSuchKey` or `NotFound` |

### 3. S3Config struct

```go
type S3Config struct {
    Endpoint  string // S3-compatible endpoint (e.g., "localhost:9000")
    AccessKey string
    SecretKey string
    Bucket    string
    Prefix    string
    UseSSL    bool
    Region    string
}
```

### 4. compose.yml — swap container

| From | To |
|------|----|
| `minio/minio:latest` | `rustfs/rustfs:latest` |
| `MINIO_ROOT_USER: minioadmin` | `RUSTFS_ACCESS_KEY: rustfsadmin` |
| `MINIO_ROOT_PASSWORD: minioadmin` | `RUSTFS_SECRET_KEY: rustfsadmin` |
| `command: server /data --console-address ":9001"` | `command: /data` |
| Volume name `minio-data` | `objectstore-data` |
| Service name `minio` | `objectstore` |

Ports stay the same: 9000 (API), 9001 (console).

### 5. Update consumers

- `examples/function/worker/main.go` — env var names change, function rename
- `examples/trigger/main.go` — workflow name `minio` references
- compose.yml — worker env vars (`MINIO_ENDPOINT` → `S3_ENDPOINT`, etc.)
- Integration test — swap testcontainer image, health check

### 6. go.mod changes

- Remove: `github.com/minio/minio-go/v7`
- Add: `github.com/aws/aws-sdk-go-v2`, `github.com/aws/aws-sdk-go-v2/service/s3`, `github.com/aws/aws-sdk-go-v2/credentials`

## What stays the same

- `ArtifactStore` interface (unchanged)
- `LocalFileStore` (unchanged)
- OTel wrapper (unchanged)
- `store.go` (unchanged)
- All function workflow code (unchanged)
