# Replace MinIO with RustFS + AWS SDK v2 Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace minio-go with AWS SDK v2 for the S3-compatible artifact store, rename MinioStore→S3Store, and swap MinIO for RustFS in compose.yml.

**Architecture:** Rewrite `minio.go` → `s3.go` using `aws-sdk-go-v2/service/s3`. Update compose.yml to use RustFS image. Update all consumers (worker, trigger, examples, docs).

**Tech Stack:** aws-sdk-go-v2, RustFS (rustfs/rustfs), testcontainers-go

**Design Doc:** `docs/plans/2026-03-27-rustfs-s3store-design.md`

---

### Task 1: Add AWS SDK v2 dependencies and create S3Store

**Files:**
- Create: `workflow/artifacts/s3.go` (replaces `minio.go`)
- Delete: `workflow/artifacts/minio.go`
- Modify: `workflow/artifacts/store.go:13` (comment update)

**Step 1: Add AWS SDK v2 dependencies**

```bash
cd /Users/jasoet/Documents/Go/go-wf
go get github.com/aws/aws-sdk-go-v2
go get github.com/aws/aws-sdk-go-v2/config
go get github.com/aws/aws-sdk-go-v2/credentials
go get github.com/aws/aws-sdk-go-v2/service/s3
go get github.com/aws/smithy-go
```

**Step 2: Create `workflow/artifacts/s3.go`**

This replaces `minio.go` entirely. The new file implements the same `ArtifactStore` interface using AWS SDK v2.

```go
package artifacts

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
)

// S3Store implements ArtifactStore using S3-compatible object storage.
// Works with any S3-compatible backend (AWS S3, RustFS, MinIO, etc.).
type S3Store struct {
	client *s3.Client
	bucket string
	prefix string
}

// S3Config contains configuration for connecting to an S3-compatible store.
type S3Config struct {
	// Endpoint is the S3-compatible endpoint (e.g., "localhost:9000")
	Endpoint string

	// AccessKey is the access key for authentication
	AccessKey string

	// SecretKey is the secret key for authentication
	SecretKey string

	// Bucket is the bucket name for storing artifacts
	Bucket string

	// Prefix is an optional prefix for all artifact keys
	Prefix string

	// UseSSL determines whether to use HTTPS
	UseSSL bool

	// Region is the bucket region (optional, defaults to "us-east-1")
	Region string
}

// NewS3Store creates a new S3-compatible artifact store.
func NewS3Store(ctx context.Context, config S3Config) (*S3Store, error) {
	region := config.Region
	if region == "" {
		region = "us-east-1"
	}

	scheme := "http"
	if config.UseSSL {
		scheme = "https"
	}
	endpoint := fmt.Sprintf("%s://%s", scheme, config.Endpoint)

	client := s3.New(s3.Options{
		Region:       region,
		BaseEndpoint: aws.String(endpoint),
		Credentials:  credentials.NewStaticCredentialsProvider(config.AccessKey, config.SecretKey, ""),
		UsePathStyle: true,
	})

	// Check if bucket exists, create if not
	_, err := client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(config.Bucket),
	})
	if err != nil {
		// Bucket doesn't exist, create it
		var nsk *types.NotFound
		var apiErr smithy.APIError
		bucketNotFound := errors.As(err, &nsk)
		if !bucketNotFound && errors.As(err, &apiErr) {
			bucketNotFound = apiErr.ErrorCode() == "NoSuchBucket" || apiErr.ErrorCode() == "NotFound" || apiErr.ErrorCode() == "404"
		}

		if !bucketNotFound {
			return nil, fmt.Errorf("failed to check bucket existence: %w", err)
		}

		_, err = client.CreateBucket(ctx, &s3.CreateBucketInput{
			Bucket: aws.String(config.Bucket),
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create bucket: %w", err)
		}
	}

	return &S3Store{
		client: client,
		bucket: config.Bucket,
		prefix: config.Prefix,
	}, nil
}

// Upload uploads an artifact to S3-compatible storage.
func (s *S3Store) Upload(ctx context.Context, metadata ArtifactMetadata, data io.Reader) error {
	if err := ValidateMetadata(metadata); err != nil {
		return err
	}

	objectName := s.objectName(metadata)

	contentType := metadata.ContentType
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(objectName),
		Body:        data,
		ContentType: aws.String(contentType),
		Metadata: map[string]string{
			"workflow-id":   metadata.WorkflowID,
			"run-id":        metadata.RunID,
			"step-name":     metadata.StepName,
			"artifact-type": metadata.Type,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to upload artifact: %w", err)
	}

	return nil
}

// Download downloads an artifact from S3-compatible storage.
func (s *S3Store) Download(ctx context.Context, metadata ArtifactMetadata) (io.ReadCloser, error) {
	if err := ValidateMetadata(metadata); err != nil {
		return nil, err
	}

	objectName := s.objectName(metadata)

	result, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(objectName),
	})
	if err != nil {
		var nsk *types.NoSuchKey
		if errors.As(err, &nsk) {
			return nil, fmt.Errorf("artifact not found: %s", metadata.Name)
		}
		return nil, fmt.Errorf("failed to download artifact: %w", err)
	}

	return result.Body, nil
}

// Delete removes an artifact from S3-compatible storage.
func (s *S3Store) Delete(ctx context.Context, metadata ArtifactMetadata) error {
	if err := ValidateMetadata(metadata); err != nil {
		return err
	}

	objectName := s.objectName(metadata)

	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(objectName),
	})
	if err != nil {
		return fmt.Errorf("failed to delete artifact: %w", err)
	}

	return nil
}

// Exists checks if an artifact exists in S3-compatible storage.
func (s *S3Store) Exists(ctx context.Context, metadata ArtifactMetadata) (bool, error) {
	if err := ValidateMetadata(metadata); err != nil {
		return false, err
	}

	objectName := s.objectName(metadata)

	_, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(objectName),
	})
	if err != nil {
		var nsk *types.NotFound
		var apiErr smithy.APIError
		notFound := errors.As(err, &nsk)
		if !notFound && errors.As(err, &apiErr) {
			notFound = apiErr.ErrorCode() == "NoSuchKey" || apiErr.ErrorCode() == "NotFound" || apiErr.ErrorCode() == "404"
		}
		if notFound {
			return false, nil
		}
		return false, fmt.Errorf("failed to check object existence: %w", err)
	}

	return true, nil
}

// List returns all artifacts matching the given prefix.
func (s *S3Store) List(ctx context.Context, prefix string) ([]ArtifactMetadata, error) {
	if err := ValidatePrefix(prefix); err != nil {
		return nil, err
	}

	objectPrefix := s.prefix
	if prefix != "" {
		objectPrefix = s.prefix + prefix
	}

	result := make([]ArtifactMetadata, 0)

	paginator := s3.NewListObjectsV2Paginator(s.client, &s3.ListObjectsV2Input{
		Bucket: aws.String(s.bucket),
		Prefix: aws.String(objectPrefix),
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list objects: %w", err)
		}

		for _, obj := range page.Contents {
			key := aws.ToString(obj.Key)
			if s.prefix != "" {
				key = strings.TrimPrefix(key, s.prefix)
			}

			parts := strings.Split(key, "/")
			if len(parts) < 4 {
				continue
			}

			result = append(result, ArtifactMetadata{
				Name:       parts[3],
				WorkflowID: parts[0],
				RunID:      parts[1],
				StepName:   parts[2],
				Size:       aws.ToInt64(obj.Size),
			})
		}
	}

	return result, nil
}

// Close cleans up resources (no-op for S3).
func (s *S3Store) Close() error {
	return nil
}

// objectName generates the full object name with prefix.
func (s *S3Store) objectName(metadata ArtifactMetadata) string {
	if s.prefix != "" {
		return s.prefix + metadata.StorageKey()
	}
	return metadata.StorageKey()
}
```

**Step 3: Delete `workflow/artifacts/minio.go`**

```bash
rm workflow/artifacts/minio.go
```

**Step 4: Update comment in `workflow/artifacts/store.go:13`**

Change:
```go
// and MinioStore for S3-compatible object storage.
```
To:
```go
// and S3Store for S3-compatible object storage.
```

**Step 5: Run unit tests**

```bash
task test:unit
```

Expected: All pass (the S3Store is only used in integration tests).

**Step 6: Run `go mod tidy`**

```bash
go mod tidy
```

This removes the `minio-go` dependency and cleans up.

---

### Task 2: Rewrite integration test for S3Store

**Files:**
- Create: `workflow/artifacts/s3_integration_test.go` (replaces `minio_integration_test.go`)
- Delete: `workflow/artifacts/minio_integration_test.go`

**Step 1: Create `workflow/artifacts/s3_integration_test.go`**

Replace the testcontainer from `minio/minio` to `rustfs/rustfs`, update env vars, health check, and all type references.

```go
//go:build integration

package artifacts

import (
	"bytes"
	"context"
	"log"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

var testS3Config S3Config

func TestMain(m *testing.M) {
	ctx := context.Background()

	req := testcontainers.ContainerRequest{
		Image:        "rustfs/rustfs:latest",
		ExposedPorts: []string{"9000/tcp"},
		Env: map[string]string{
			"RUSTFS_ACCESS_KEY": "rustfsadmin",
			"RUSTFS_SECRET_KEY": "rustfsadmin",
		},
		Cmd:        []string{"/data"},
		WaitingFor: wait.ForListeningPort("9000").WithStartupTimeout(60 * time.Second),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		log.Fatalf("failed to start object store container: %v", err)
	}

	host, err := container.Host(ctx)
	if err != nil {
		log.Fatalf("failed to get container host: %v", err)
	}

	port, err := container.MappedPort(ctx, "9000")
	if err != nil {
		log.Fatalf("failed to get mapped port: %v", err)
	}

	testS3Config = S3Config{
		Endpoint:  host + ":" + port.Port(),
		AccessKey: "rustfsadmin",
		SecretKey: "rustfsadmin",
		Bucket:    "test-artifacts",
		Prefix:    "workflows/",
		UseSSL:    false,
		Region:    "us-east-1",
	}

	code := m.Run()

	if err := container.Terminate(ctx); err != nil {
		log.Printf("failed to terminate container: %v", err)
	}

	os.Exit(code)
}

func TestS3Store_UploadDownload(t *testing.T) {
	ctx := context.Background()

	store, err := NewS3Store(ctx, testS3Config)
	require.NoError(t, err)
	defer store.Close()

	metadata := ArtifactMetadata{
		Name:       "test-file",
		Path:       "/test/file.txt",
		Type:       "file",
		WorkflowID: "workflow-123",
		RunID:      "run-456",
		StepName:   "build",
	}

	content := []byte("test content for s3 store")
	err = store.Upload(ctx, metadata, bytes.NewReader(content))
	require.NoError(t, err)

	exists, err := store.Exists(ctx, metadata)
	require.NoError(t, err)
	assert.True(t, exists)

	reader, err := store.Download(ctx, metadata)
	require.NoError(t, err)
	defer reader.Close()

	buf := new(bytes.Buffer)
	_, err = buf.ReadFrom(reader)
	require.NoError(t, err)
	assert.Equal(t, content, buf.Bytes())
}

func TestS3Store_Delete(t *testing.T) {
	ctx := context.Background()

	store, err := NewS3Store(ctx, testS3Config)
	require.NoError(t, err)
	defer store.Close()

	metadata := ArtifactMetadata{
		Name:       "test-file",
		Path:       "/test/file.txt",
		Type:       "file",
		WorkflowID: "workflow-123",
		RunID:      "run-456",
		StepName:   "build",
	}

	err = store.Upload(ctx, metadata, bytes.NewReader([]byte("test")))
	require.NoError(t, err)

	err = store.Delete(ctx, metadata)
	require.NoError(t, err)

	exists, err := store.Exists(ctx, metadata)
	require.NoError(t, err)
	assert.False(t, exists)
}

func TestS3Store_List(t *testing.T) {
	ctx := context.Background()

	store, err := NewS3Store(ctx, testS3Config)
	require.NoError(t, err)
	defer store.Close()

	artifacts := []ArtifactMetadata{
		{Name: "file1", WorkflowID: "workflow-123", RunID: "run-456", StepName: "build", Type: "file"},
		{Name: "file2", WorkflowID: "workflow-123", RunID: "run-456", StepName: "test", Type: "file"},
		{Name: "file3", WorkflowID: "workflow-123", RunID: "run-789", StepName: "build", Type: "file"},
	}

	for _, metadata := range artifacts {
		err := store.Upload(ctx, metadata, bytes.NewReader([]byte("test")))
		require.NoError(t, err)
	}

	listed, err := store.List(ctx, "workflow-123/run-456/")
	require.NoError(t, err)
	assert.Len(t, listed, 2)

	listed, err = store.List(ctx, "workflow-123/")
	require.NoError(t, err)
	assert.Len(t, listed, 3)
}

func TestS3Store_UploadDownloadActivities(t *testing.T) {
	ctx := context.Background()

	store, err := NewS3Store(ctx, testS3Config)
	require.NoError(t, err)
	defer store.Close()

	content := []byte("test file content for activities")
	tmpFile := t.TempDir() + "/test-file.txt"
	err = os.WriteFile(tmpFile, content, 0o600)
	require.NoError(t, err)

	metadata := ArtifactMetadata{
		Name:       "activity-test",
		Path:       tmpFile,
		Type:       "file",
		WorkflowID: "workflow-activity",
		RunID:      "run-activity",
		StepName:   "upload-step",
	}

	uploadInput := UploadArtifactInput{
		Metadata:   metadata,
		SourcePath: tmpFile,
	}

	err = UploadArtifactActivity(ctx, store, uploadInput)
	require.NoError(t, err)

	destFile := t.TempDir() + "/downloaded-file.txt"
	downloadInput := DownloadArtifactInput{
		Metadata: metadata,
		DestPath: destFile,
	}

	err = DownloadArtifactActivity(ctx, store, downloadInput)
	require.NoError(t, err)

	downloaded, err := os.ReadFile(destFile)
	require.NoError(t, err)
	assert.Equal(t, content, downloaded)
}

func TestS3Store_CleanupWorkflow(t *testing.T) {
	ctx := context.Background()

	store, err := NewS3Store(ctx, testS3Config)
	require.NoError(t, err)
	defer store.Close()

	workflowID := "workflow-cleanup"
	runID := "run-cleanup"

	artifacts := []ArtifactMetadata{
		{Name: "file1", WorkflowID: workflowID, RunID: runID, StepName: "step1", Type: "file"},
		{Name: "file2", WorkflowID: workflowID, RunID: runID, StepName: "step2", Type: "file"},
		{Name: "file3", WorkflowID: workflowID, RunID: runID, StepName: "step3", Type: "file"},
	}

	for _, metadata := range artifacts {
		err := store.Upload(ctx, metadata, bytes.NewReader([]byte("test")))
		require.NoError(t, err)
	}

	for _, metadata := range artifacts {
		exists, err := store.Exists(ctx, metadata)
		require.NoError(t, err)
		assert.True(t, exists)
	}

	err = CleanupWorkflowArtifacts(ctx, store, workflowID, runID)
	require.NoError(t, err)

	for _, metadata := range artifacts {
		exists, err := store.Exists(ctx, metadata)
		require.NoError(t, err)
		assert.False(t, exists)
	}
}
```

**Step 2: Delete `workflow/artifacts/minio_integration_test.go`**

```bash
rm workflow/artifacts/minio_integration_test.go
```

**Step 3: Run unit tests (sanity check)**

```bash
task test:unit
```

Expected: All pass.

**Step 4: Run integration tests**

```bash
task test:run -- -run TestS3Store ./workflow/artifacts/... -tags integration
```

Expected: All 5 tests pass.

---

### Task 3: Update compose.yml and consumer code

**Files:**
- Modify: `compose.yml` — swap MinIO for RustFS
- Modify: `examples/function/worker/main.go` — update env vars and function names
- Modify: `examples/trigger/main.go` — update workflow name reference

**Step 1: Update compose.yml**

Replace the `minio` service with `objectstore`:

```yaml
  objectstore:
    image: rustfs/rustfs:latest
    ports:
      - "127.0.0.1:9000:9000"
      - "127.0.0.1:9001:9001"
    environment:
      RUSTFS_ACCESS_KEY: rustfsadmin
      RUSTFS_SECRET_KEY: rustfsadmin
    command: /data
    volumes:
      - objectstore-data:/data
    healthcheck:
      test: ["CMD-SHELL", "wget -qO- http://localhost:9000/ || exit 1"]
      interval: 10s
      timeout: 5s
      retries: 5
```

Update `function-worker` service env vars:
```yaml
    environment:
      TEMPORAL_HOST_PORT: temporal:7233
      S3_ENDPOINT: objectstore:9000
      S3_ACCESS_KEY: rustfsadmin
      S3_SECRET_KEY: rustfsadmin
    depends_on:
      temporal:
        condition: service_healthy
      objectstore:
        condition: service_healthy
```

Update volumes section:
```yaml
volumes:
  postgresql-data:
  objectstore-data:
  artifact-data:
```

**Step 2: Update `examples/function/worker/main.go`**

Replace `createMinioArtifactStore` with `createS3ArtifactStore`:
- Read `S3_ENDPOINT` instead of `MINIO_ENDPOINT`
- Read `S3_ACCESS_KEY` instead of `MINIO_ACCESS_KEY`
- Read `S3_SECRET_KEY` instead of `MINIO_SECRET_KEY`
- Use `artifacts.NewS3Store` instead of `artifacts.NewMinioStore`
- Use `artifacts.S3Config` instead of `artifacts.MinioConfig`
- Default credentials: `rustfsadmin` instead of `minioadmin`
- Update log messages from "MinIO" to "S3 object store"
- Update variable names from `minioStore` to `s3Store`
- Keep workflow name `ArtifactDAGWorkflow-MinIO` → rename to `ArtifactDAGWorkflow-S3`

**Step 3: Update `examples/trigger/main.go`**

Change workflow name reference:
- `"demo-fn-dag-artifact-minio-"` → `"demo-fn-dag-artifact-s3-"`
- `"ArtifactDAGWorkflow-MinIO"` → `"ArtifactDAGWorkflow-S3"` (if referenced)

**Step 4: Run unit tests**

```bash
task test:unit
```

Expected: All pass.

---

### Task 4: Update documentation and remaining references

**Files:**
- Modify: `INSTRUCTION.md` — update MinIO references
- Modify: `README.md` — update MinIO references
- Modify: `container/README.md` — if any MinIO refs
- Modify: `examples/container/README.md` — if any MinIO refs
- Modify: `examples/function/README.md` — if any MinIO refs
- Modify: `workflow/artifacts/store.go:13` — comment update

**Step 1: Update all documentation**

Search and replace in documentation files:
- "MinIO" → "S3-compatible object storage" or "RustFS" (context-dependent)
- "minio" → appropriate replacement
- `minio-go` → `aws-sdk-go-v2`
- `MinioStore` → `S3Store`
- `MinioConfig` → `S3Config`
- `NewMinioStore` → `NewS3Store`
- Keep "MinIO" where it refers to MinIO as one of the compatible backends

**Step 2: Run all tests**

```bash
task test:unit
task lint
```

Expected: All pass, 0 lint issues.

---

### Task 5: Clean up and commit

**Step 1: Run `go mod tidy`**

```bash
go mod tidy
```

**Step 2: Verify no stale minio-go imports**

```bash
grep -r "minio-go" --include="*.go" .
# Should return nothing
```

**Step 3: Run full verification**

```bash
task test:unit
task lint
```

**Step 4: Commit**

```bash
git add -A
git commit -m "refactor(artifacts): replace minio-go with AWS SDK v2, swap MinIO for RustFS

- Rename MinioStore/MinioConfig to S3Store/S3Config
- Replace minio-go client with aws-sdk-go-v2/service/s3
- Swap MinIO container for RustFS in compose.yml
- Update env vars: MINIO_* -> S3_*, credentials -> rustfsadmin
- Update integration tests to use RustFS testcontainer
- Works with any S3-compatible backend (RustFS, MinIO, AWS S3)"
```

---

### Task 6: Restart local environment and verify

**Step 1: Clean and restart**

```bash
task local:stop
task local:clean
task local:start
```

**Step 2: Verify all containers are healthy**

```bash
podman ps --format "{{.Names}}\t{{.Status}}"
```

Expected: All services up including `go-wf_objectstore_1`.

**Step 3: Check Temporal UI for workflow status**

Open http://localhost:8233 and verify:
- All workflows running/completing
- `ArtifactDAGWorkflow-S3` completing successfully
- No failures
