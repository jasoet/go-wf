//go:build integration

package store

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
		Bucket:    "test-store",
		Prefix:    "data/",
		UseSSL:    false,
		Region:    "us-east-1",
	}

	code := m.Run()

	if err := container.Terminate(ctx); err != nil {
		log.Printf("failed to terminate container: %v", err)
	}

	os.Exit(code)
}

func TestS3Store_RoundTrip(t *testing.T) {
	ctx := context.Background()

	// Create S3 store
	rawStore, err := NewS3Store(ctx, testS3Config)
	require.NoError(t, err)
	defer rawStore.Close()

	key := "test/round-trip.txt"
	content := []byte("hello s3 store")

	// Upload
	err = rawStore.Upload(ctx, key, bytes.NewReader(content))
	require.NoError(t, err)

	// Exists (true)
	exists, err := rawStore.Exists(ctx, key)
	require.NoError(t, err)
	assert.True(t, exists)

	// Download
	reader, err := rawStore.Download(ctx, key)
	require.NoError(t, err)
	defer reader.Close()

	buf := new(bytes.Buffer)
	_, err = buf.ReadFrom(reader)
	require.NoError(t, err)
	assert.Equal(t, content, buf.Bytes())

	// List
	keys, err := rawStore.List(ctx, "test/")
	require.NoError(t, err)
	assert.Contains(t, keys, key)

	// Delete
	err = rawStore.Delete(ctx, key)
	require.NoError(t, err)

	// Exists (false)
	exists, err = rawStore.Exists(ctx, key)
	require.NoError(t, err)
	assert.False(t, exists)
}

func TestS3Store_ListMultiple(t *testing.T) {
	ctx := context.Background()

	rawStore, err := NewS3Store(ctx, testS3Config)
	require.NoError(t, err)
	defer rawStore.Close()

	// Upload multiple objects under different prefixes
	objects := map[string]string{
		"proj/a/file1.txt": "content1",
		"proj/a/file2.txt": "content2",
		"proj/b/file3.txt": "content3",
	}

	for k, v := range objects {
		err := rawStore.Upload(ctx, k, bytes.NewReader([]byte(v)))
		require.NoError(t, err)
	}

	// List with prefix "proj/a/"
	keys, err := rawStore.List(ctx, "proj/a/")
	require.NoError(t, err)
	assert.Len(t, keys, 2)

	// List with prefix "proj/"
	keys, err = rawStore.List(ctx, "proj/")
	require.NoError(t, err)
	assert.Len(t, keys, 3)

	// Cleanup
	for k := range objects {
		err := rawStore.Delete(ctx, k)
		require.NoError(t, err)
	}
}

func TestS3Store_DownloadNotFound(t *testing.T) {
	ctx := context.Background()

	rawStore, err := NewS3Store(ctx, testS3Config)
	require.NoError(t, err)
	defer rawStore.Close()

	_, err = rawStore.Download(ctx, "nonexistent/key.txt")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}
