package artifacts

import (
	"bytes"
	"context"
	"fmt"
	"io"
)

// UploadBytes uploads in-memory data as an artifact.
func UploadBytes(ctx context.Context, store ArtifactStore, metadata ArtifactMetadata, data []byte) error {
	metadata.Size = int64(len(data))
	if metadata.ContentType == "" {
		metadata.ContentType = "application/octet-stream"
	}
	return store.Upload(ctx, metadata, bytes.NewReader(data))
}

// DownloadBytes downloads an artifact as in-memory bytes.
func DownloadBytes(ctx context.Context, store ArtifactStore, metadata ArtifactMetadata) ([]byte, error) {
	reader, err := store.Download(ctx, metadata)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = reader.Close() //nolint:errcheck // best-effort close on read path
	}()

	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read artifact data: %w", err)
	}
	return data, nil
}
