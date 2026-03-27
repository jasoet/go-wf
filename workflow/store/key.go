package store

import "strings"

// KeyBuilder provides composable key generation for storage keys.
// Keys are built by appending path segments and joining them with slashes.
// KeyBuilder is immutable — each With* method returns a new instance,
// making it safe to branch from a shared base.
type KeyBuilder struct {
	parts []string
}

// NewKeyBuilder creates a new empty KeyBuilder.
func NewKeyBuilder() *KeyBuilder {
	return &KeyBuilder{}
}

// withPart returns a new KeyBuilder with the given segment appended.
func (kb *KeyBuilder) withPart(part string) *KeyBuilder {
	newParts := make([]string, len(kb.parts), len(kb.parts)+1)
	copy(newParts, kb.parts)
	return &KeyBuilder{parts: append(newParts, part)}
}

// WithWorkflow appends a workflow ID segment.
func (kb *KeyBuilder) WithWorkflow(id string) *KeyBuilder {
	return kb.withPart(id)
}

// WithRun appends a run ID segment.
func (kb *KeyBuilder) WithRun(id string) *KeyBuilder {
	return kb.withPart(id)
}

// WithStep appends a step name segment.
func (kb *KeyBuilder) WithStep(name string) *KeyBuilder {
	return kb.withPart(name)
}

// WithName appends a name segment.
func (kb *KeyBuilder) WithName(name string) *KeyBuilder {
	return kb.withPart(name)
}

// Build joins all segments with slashes and returns the key.
func (kb *KeyBuilder) Build() string {
	return strings.Join(kb.parts, "/")
}
