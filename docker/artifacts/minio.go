package artifacts

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// MinioStore implements ArtifactStore using Minio/S3-compatible storage.
type MinioStore struct {
	client *minio.Client
	bucket string
	prefix string
}

// MinioConfig contains configuration for connecting to Minio.
type MinioConfig struct {
	// Endpoint is the Minio server endpoint (e.g., "localhost:9000")
	Endpoint string

	// AccessKey is the Minio access key
	AccessKey string

	// SecretKey is the Minio secret key
	SecretKey string

	// Bucket is the bucket name for storing artifacts
	Bucket string

	// Prefix is an optional prefix for all artifact keys
	Prefix string

	// UseSSL determines whether to use HTTPS
	UseSSL bool

	// Region is the bucket region (optional)
	Region string
}

// NewMinioStore creates a new Minio artifact store.
func NewMinioStore(ctx context.Context, config MinioConfig) (*MinioStore, error) {
	// Initialize Minio client
	client, err := minio.New(config.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(config.AccessKey, config.SecretKey, ""),
		Secure: config.UseSSL,
		Region: config.Region,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Minio client: %w", err)
	}

	// Check if bucket exists, create if not
	exists, err := client.BucketExists(ctx, config.Bucket)
	if err != nil {
		return nil, fmt.Errorf("failed to check bucket existence: %w", err)
	}

	if !exists {
		err = client.MakeBucket(ctx, config.Bucket, minio.MakeBucketOptions{
			Region: config.Region,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create bucket: %w", err)
		}
	}

	return &MinioStore{
		client: client,
		bucket: config.Bucket,
		prefix: config.Prefix,
	}, nil
}

// Upload uploads an artifact to Minio.
func (s *MinioStore) Upload(ctx context.Context, metadata ArtifactMetadata, data io.Reader) error {
	objectName := s.objectName(metadata)

	// Determine content type
	contentType := metadata.ContentType
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	// Upload object
	_, err := s.client.PutObject(ctx, s.bucket, objectName, data, -1, minio.PutObjectOptions{
		ContentType: contentType,
		UserMetadata: map[string]string{
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

// Download downloads an artifact from Minio.
func (s *MinioStore) Download(ctx context.Context, metadata ArtifactMetadata) (io.ReadCloser, error) {
	objectName := s.objectName(metadata)

	object, err := s.client.GetObject(ctx, s.bucket, objectName, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to download artifact: %w", err)
	}

	// Verify object exists by checking stat
	_, err = object.Stat()
	if err != nil {
		_ = object.Close()
		return nil, fmt.Errorf("artifact not found: %s", metadata.Name)
	}

	return object, nil
}

// Delete removes an artifact from Minio.
func (s *MinioStore) Delete(ctx context.Context, metadata ArtifactMetadata) error {
	objectName := s.objectName(metadata)

	err := s.client.RemoveObject(ctx, s.bucket, objectName, minio.RemoveObjectOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete artifact: %w", err)
	}

	return nil
}

// Exists checks if an artifact exists in Minio.
func (s *MinioStore) Exists(ctx context.Context, metadata ArtifactMetadata) (bool, error) {
	objectName := s.objectName(metadata)

	_, err := s.client.StatObject(ctx, s.bucket, objectName, minio.StatObjectOptions{})
	if err != nil {
		// Check if error is "not found"
		errResponse := minio.ToErrorResponse(err)
		if errResponse.Code == "NoSuchKey" {
			return false, nil
		}
		return false, fmt.Errorf("failed to stat object: %w", err)
	}

	return true, nil
}

// List returns all artifacts matching the given prefix.
func (s *MinioStore) List(ctx context.Context, prefix string) ([]ArtifactMetadata, error) {
	objectPrefix := s.prefix
	if prefix != "" {
		objectPrefix = s.prefix + prefix
	}

	var artifacts []ArtifactMetadata

	// List objects with prefix
	objectCh := s.client.ListObjects(ctx, s.bucket, minio.ListObjectsOptions{
		Prefix:    objectPrefix,
		Recursive: true,
	})

	for object := range objectCh {
		if object.Err != nil {
			return nil, fmt.Errorf("failed to list objects: %w", object.Err)
		}

		// Parse object key: prefix/workflow_id/run_id/step_name/artifact_name
		key := object.Key
		if s.prefix != "" {
			key = strings.TrimPrefix(key, s.prefix)
		}

		parts := strings.Split(key, "/")
		if len(parts) < 4 {
			continue // Skip invalid keys
		}

		artifacts = append(artifacts, ArtifactMetadata{
			Name:        parts[3],
			WorkflowID:  parts[0],
			RunID:       parts[1],
			StepName:    parts[2],
			Size:        object.Size,
			ContentType: object.ContentType,
		})
	}

	return artifacts, nil
}

// Close cleans up resources (no-op for Minio).
func (s *MinioStore) Close() error {
	return nil
}

// objectName generates the full object name with prefix.
func (s *MinioStore) objectName(metadata ArtifactMetadata) string {
	if s.prefix != "" {
		return s.prefix + metadata.StorageKey()
	}
	return metadata.StorageKey()
}
