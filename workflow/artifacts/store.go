package artifacts

import (
	"context"
	"fmt"
	"io"
	"regexp"
	"strings"
)

// ArtifactStore is an interface for storing and retrieving artifacts.
// Implementations include LocalFileStore for local filesystem storage
// and MinioStore for S3-compatible object storage.
type ArtifactStore interface {
	// Upload uploads an artifact to the store.
	// The artifact metadata includes name, path, and type information.
	// The data reader contains the artifact content.
	Upload(ctx context.Context, metadata ArtifactMetadata, data io.Reader) error

	// Download downloads an artifact from the store.
	// Returns a reader for the artifact content that must be closed by the caller.
	Download(ctx context.Context, metadata ArtifactMetadata) (io.ReadCloser, error)

	// Delete removes an artifact from the store.
	Delete(ctx context.Context, metadata ArtifactMetadata) error

	// Exists checks if an artifact exists in the store.
	Exists(ctx context.Context, metadata ArtifactMetadata) (bool, error)

	// List returns all artifacts matching the given prefix.
	List(ctx context.Context, prefix string) ([]ArtifactMetadata, error)

	// Close cleans up any resources used by the store.
	Close() error
}

// ArtifactMetadata contains metadata about an artifact.
type ArtifactMetadata struct {
	// Name is the artifact identifier
	Name string `json:"name"`

	// Path is the local file/directory path
	Path string `json:"path"`

	// Type can be "file", "directory", or "archive"
	Type string `json:"type"`

	// WorkflowID is the Temporal workflow ID
	WorkflowID string `json:"workflow_id"`

	// RunID is the Temporal run ID
	RunID string `json:"run_id"`

	// StepName is the DAG node name that produced this artifact
	StepName string `json:"step_name"`

	// ContentType is the MIME type of the artifact
	ContentType string `json:"content_type,omitempty"`

	// Size is the artifact size in bytes
	Size int64 `json:"size,omitempty"`
}

// safeNamePattern allows alphanumeric, hyphens, underscores, dots.
var safeNamePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]*$`)

// safeWorkflowIDPattern also allows colons for Temporal workflow IDs with ISO 8601 timestamps.
var safeWorkflowIDPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._:-]*$`)

// ValidateMetadata checks that metadata fields are safe for use as path/key components.
func ValidateMetadata(m ArtifactMetadata) error {
	fields := []struct {
		name, value string
		pattern     *regexp.Regexp
	}{
		{"WorkflowID", m.WorkflowID, safeWorkflowIDPattern},
		{"RunID", m.RunID, safeNamePattern},
		{"StepName", m.StepName, safeNamePattern},
		{"Name", m.Name, safeNamePattern},
	}
	for _, f := range fields {
		if f.value == "" {
			return fmt.Errorf("invalid metadata: %s must not be empty", f.name)
		}
		if strings.ContainsAny(f.value, "/\\\x00") {
			return fmt.Errorf("invalid metadata: %s contains forbidden characters", f.name)
		}
		if strings.Contains(f.value, "..") {
			return fmt.Errorf("invalid metadata: %s contains path traversal sequence", f.name)
		}
		if !f.pattern.MatchString(f.value) {
			return fmt.Errorf("invalid metadata: %s contains invalid characters", f.name)
		}
	}
	return nil
}

// ValidatePrefix checks that a list prefix is safe for use in path/key lookups.
func ValidatePrefix(prefix string) error {
	if strings.Contains(prefix, "..") {
		return fmt.Errorf("invalid prefix: contains path traversal sequence")
	}
	if strings.ContainsAny(prefix, "\\\x00") {
		return fmt.Errorf("invalid prefix: contains forbidden characters")
	}
	return nil
}

// StorageKey generates a storage key for an artifact,
// using the format: workflow_id/run_id/step_name/artifact_name.
func (m ArtifactMetadata) StorageKey() string {
	return m.WorkflowID + "/" + m.RunID + "/" + m.StepName + "/" + m.Name
}

// ArtifactRef defines a reference to an artifact on a workflow node.
// Used by both docker and function DAG nodes to declare input/output artifacts.
type ArtifactRef struct {
	// Name is the artifact identifier
	Name string `json:"name" validate:"required"`

	// Path is the file/directory path (used for file-based artifacts)
	Path string `json:"path,omitempty"`

	// Type can be "file", "directory", "archive", or "bytes"
	Type string `json:"type" validate:"required,oneof=file directory archive bytes"`

	// Optional indicates if the artifact is optional
	Optional bool `json:"optional"`
}

// ArtifactConfig contains configuration for artifact storage.
type ArtifactConfig struct {
	// Store is the artifact storage backend
	Store ArtifactStore

	// WorkflowID is the current workflow ID
	WorkflowID string

	// RunID is the current run ID
	RunID string

	// EnableCleanup determines if artifacts should be cleaned up after workflow completion
	EnableCleanup bool

	// RetentionDays is how long to keep artifacts (0 = forever)
	RetentionDays int
}
