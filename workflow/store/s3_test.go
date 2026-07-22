package store

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsS3NotFound(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil error", err: nil, want: false},
		{name: "generic error", err: errors.New("nope"), want: false},
		{name: "types.NotFound", err: &types.NotFound{}, want: true},
		{name: "wrapped types.NotFound", err: fmt.Errorf("head: %w", &types.NotFound{}), want: true},
		{name: "api NoSuchBucket", err: &smithy.GenericAPIError{Code: "NoSuchBucket"}, want: true},
		{name: "api NotFound", err: &smithy.GenericAPIError{Code: "NotFound"}, want: true},
		{name: "api 404", err: &smithy.GenericAPIError{Code: "404"}, want: true},
		{name: "api other code", err: &smithy.GenericAPIError{Code: "AccessDenied"}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isS3NotFound(tt.err))
		})
	}
}

func TestIsS3ObjectNotFound(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil error", err: nil, want: false},
		{name: "generic error", err: errors.New("nope"), want: false},
		{name: "types.NotFound", err: &types.NotFound{}, want: true},
		{name: "types.NoSuchKey", err: &types.NoSuchKey{}, want: true},
		{name: "wrapped types.NoSuchKey", err: fmt.Errorf("get: %w", &types.NoSuchKey{}), want: true},
		{name: "api NoSuchKey", err: &smithy.GenericAPIError{Code: "NoSuchKey"}, want: true},
		{name: "api NotFound", err: &smithy.GenericAPIError{Code: "NotFound"}, want: true},
		{name: "api 404", err: &smithy.GenericAPIError{Code: "404"}, want: true},
		{name: "api other code", err: &smithy.GenericAPIError{Code: "AccessDenied"}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isS3ObjectNotFound(tt.err))
		})
	}
}

func TestS3Store_FullKey(t *testing.T) {
	tests := []struct {
		name   string
		prefix string
		key    string
		want   string
	}{
		{name: "no prefix", prefix: "", key: "a/b.txt", want: "a/b.txt"},
		{name: "with prefix", prefix: "tenant-1/", key: "a/b.txt", want: "tenant-1/a/b.txt"},
		{name: "empty key with prefix", prefix: "tenant-1/", key: "", want: "tenant-1/"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &S3Store{prefix: tt.prefix}
			assert.Equal(t, tt.want, s.fullKey(tt.key))
		})
	}
}

func TestS3Store_Close(t *testing.T) {
	s := &S3Store{}
	assert.NoError(t, s.Close())
}

func TestNewS3Store_UnreachableEndpoint(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	_, err := NewS3Store(ctx, S3Config{
		Endpoint:  "127.0.0.1:1",
		AccessKey: "test",
		SecretKey: "test",
		Bucket:    "bucket",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to check bucket existence")
}
