package store

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

// S3Config contains configuration for connecting to an S3-compatible storage service.
type S3Config struct {
	// Endpoint is the S3-compatible server endpoint (e.g., "localhost:9000")
	Endpoint string

	// AccessKey is the access key ID
	AccessKey string

	// SecretKey is the secret access key
	SecretKey string

	// Bucket is the bucket name for storing data
	Bucket string

	// Prefix is an optional prefix for all object keys
	Prefix string

	// UseSSL determines whether to use HTTPS
	UseSSL bool

	// Region is the bucket region (defaults to "us-east-1" if empty)
	Region string
}

// S3Store implements RawStore using S3-compatible storage via AWS SDK v2.
type S3Store struct {
	client *s3.Client
	bucket string
	prefix string
}

// NewS3Store creates a new S3 raw store.
// It auto-creates the bucket if it does not exist.
func NewS3Store(ctx context.Context, cfg S3Config) (RawStore, error) {
	region := cfg.Region
	if region == "" {
		region = "us-east-1"
	}

	scheme := "http"
	if cfg.UseSSL {
		scheme = "https"
	}
	endpointURL := fmt.Sprintf("%s://%s", scheme, cfg.Endpoint)

	client := s3.New(s3.Options{
		Region:       region,
		BaseEndpoint: aws.String(endpointURL),
		Credentials:  credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, ""),
		UsePathStyle: true,
	})

	// Check if bucket exists, create if not
	_, err := client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(cfg.Bucket),
	})
	if err != nil {
		if isS3NotFound(err) {
			_, createErr := client.CreateBucket(ctx, &s3.CreateBucketInput{
				Bucket: aws.String(cfg.Bucket),
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
		bucket: cfg.Bucket,
		prefix: cfg.Prefix,
	}, nil
}

// Upload stores data under the given key.
func (s *S3Store) Upload(ctx context.Context, key string, data io.Reader) error {
	objectKey := s.fullKey(key)

	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(objectKey),
		Body:   data,
	})
	if err != nil {
		return fmt.Errorf("failed to upload object: %w", err)
	}

	return nil
}

// Download retrieves data for the given key.
// The caller must close the returned ReadCloser.
func (s *S3Store) Download(ctx context.Context, key string) (io.ReadCloser, error) {
	objectKey := s.fullKey(key)

	result, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(objectKey),
	})
	if err != nil {
		if isS3ObjectNotFound(err) {
			return nil, fmt.Errorf("object not found: %s", key)
		}
		return nil, fmt.Errorf("failed to download object: %w", err)
	}

	return result.Body, nil
}

// Delete removes the data stored under the given key.
func (s *S3Store) Delete(ctx context.Context, key string) error {
	objectKey := s.fullKey(key)

	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(objectKey),
	})
	if err != nil {
		return fmt.Errorf("failed to delete object: %w", err)
	}

	return nil
}

// Exists checks whether data exists under the given key.
func (s *S3Store) Exists(ctx context.Context, key string) (bool, error) {
	objectKey := s.fullKey(key)

	_, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(objectKey),
	})
	if err != nil {
		if isS3ObjectNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to check object existence: %w", err)
	}

	return true, nil
}

// List returns all keys matching the given prefix.
// The returned keys have the store's prefix stripped.
func (s *S3Store) List(ctx context.Context, prefix string) ([]string, error) {
	objectPrefix := s.fullKey(prefix)

	var keys []string

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
			keys = append(keys, key)
		}
	}

	return keys, nil
}

// Close releases any resources held by the store (no-op for S3).
func (s *S3Store) Close() error {
	return nil
}

// fullKey returns the object key with the store prefix prepended.
func (s *S3Store) fullKey(key string) string {
	if s.prefix != "" {
		return s.prefix + key
	}
	return key
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
