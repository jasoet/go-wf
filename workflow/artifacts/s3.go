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

// S3Store implements ArtifactStore using S3-compatible storage via AWS SDK v2.
type S3Store struct {
	client *s3.Client
	bucket string
	prefix string
}

// S3Config contains configuration for connecting to an S3-compatible storage service.
type S3Config struct {
	// Endpoint is the S3-compatible server endpoint (e.g., "localhost:9000")
	Endpoint string

	// AccessKey is the access key ID
	AccessKey string

	// SecretKey is the secret access key
	SecretKey string

	// Bucket is the bucket name for storing artifacts
	Bucket string

	// Prefix is an optional prefix for all artifact keys
	Prefix string

	// UseSSL determines whether to use HTTPS
	UseSSL bool

	// Region is the bucket region (defaults to "us-east-1" if empty)
	Region string
}

// NewS3Store creates a new S3 artifact store.
func NewS3Store(ctx context.Context, config S3Config) (*S3Store, error) {
	region := config.Region
	if region == "" {
		region = "us-east-1"
	}

	// Build endpoint URL
	scheme := "http"
	if config.UseSSL {
		scheme = "https"
	}
	endpointURL := fmt.Sprintf("%s://%s", scheme, config.Endpoint)

	// Create S3 client
	client := s3.New(s3.Options{
		Region:       region,
		BaseEndpoint: aws.String(endpointURL),
		Credentials:  credentials.NewStaticCredentialsProvider(config.AccessKey, config.SecretKey, ""),
		UsePathStyle: true,
	})

	// Check if bucket exists, create if not
	_, err := client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(config.Bucket),
	})
	if err != nil {
		if isS3NotFound(err) {
			_, createErr := client.CreateBucket(ctx, &s3.CreateBucketInput{
				Bucket: aws.String(config.Bucket),
			})
			if createErr != nil {
				return nil, fmt.Errorf("failed to create bucket: %w", createErr)
			}
		} else {
			return nil, fmt.Errorf("failed to check bucket existence: %w", err)
		}
	}

	return &S3Store{
		client: client,
		bucket: config.Bucket,
		prefix: config.Prefix,
	}, nil
}

// Upload uploads an artifact to S3.
func (s *S3Store) Upload(ctx context.Context, metadata ArtifactMetadata, data io.Reader) error {
	if err := ValidateMetadata(metadata); err != nil {
		return err
	}

	objectName := s.objectName(metadata)

	// Determine content type
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

// Download downloads an artifact from S3.
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
		var noSuchKey *types.NoSuchKey
		if errors.As(err, &noSuchKey) {
			return nil, fmt.Errorf("artifact not found: %s", metadata.Name)
		}
		return nil, fmt.Errorf("failed to download artifact: %w", err)
	}

	return result.Body, nil
}

// Delete removes an artifact from S3.
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

// Exists checks if an artifact exists in S3.
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
		if isS3ObjectNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to stat object: %w", err)
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

	artifacts := make([]ArtifactMetadata, 0)

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
				continue // Skip invalid keys
			}

			artifacts = append(artifacts, ArtifactMetadata{
				Name:       parts[3],
				WorkflowID: parts[0],
				RunID:      parts[1],
				StepName:   parts[2],
				Size:       aws.ToInt64(obj.Size),
			})
		}
	}

	return artifacts, nil
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

// isS3NotFound checks if an error indicates a bucket was not found.
func isS3NotFound(err error) bool {
	var notFound *types.NotFound
	if errors.As(err, &notFound) {
		return true
	}

	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.ErrorCode() {
		case "NoSuchBucket", "NotFound", "404":
			return true
		}
	}

	return false
}

// isS3ObjectNotFound checks if an error indicates an object was not found.
func isS3ObjectNotFound(err error) bool {
	var notFound *types.NotFound
	if errors.As(err, &notFound) {
		return true
	}

	var noSuchKey *types.NoSuchKey
	if errors.As(err, &noSuchKey) {
		return true
	}

	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.ErrorCode() {
		case "NoSuchKey", "NotFound", "404":
			return true
		}
	}

	return false
}
