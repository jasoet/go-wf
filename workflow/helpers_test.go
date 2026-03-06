package workflow

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSubstituteTemplate(t *testing.T) {
	tests := []struct {
		name     string
		tmpl     string
		item     string
		index    int
		params   map[string]string
		expected string
	}{
		{
			name:     "item replacement",
			tmpl:     "process-{{item}}",
			item:     "hello",
			index:    0,
			params:   nil,
			expected: "process-hello",
		},
		{
			name:     "index replacement",
			tmpl:     "step-{{index}}",
			item:     "",
			index:    3,
			params:   nil,
			expected: "step-3",
		},
		{
			name:     "param replacement with dot syntax",
			tmpl:     "image:{{.version}}",
			item:     "",
			index:    0,
			params:   map[string]string{"version": "1.0"},
			expected: "image:1.0",
		},
		{
			name:     "param replacement without dot syntax",
			tmpl:     "image:{{version}}",
			item:     "",
			index:    0,
			params:   map[string]string{"version": "1.0"},
			expected: "image:1.0",
		},
		{
			name:     "combined replacements",
			tmpl:     "{{item}}-{{index}}-{{.env}}",
			item:     "app",
			index:    2,
			params:   map[string]string{"env": "prod"},
			expected: "app-2-prod",
		},
		{
			name:     "no replacements needed",
			tmpl:     "plain-string",
			item:     "unused",
			index:    0,
			params:   nil,
			expected: "plain-string",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SubstituteTemplate(tt.tmpl, tt.item, tt.index, tt.params)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGenerateParameterCombinations(t *testing.T) {
	tests := []struct {
		name          string
		params        map[string][]string
		expectedNil   bool
		expectedCount int
	}{
		{
			name:        "nil params",
			params:      nil,
			expectedNil: true,
		},
		{
			name:        "empty params",
			params:      map[string][]string{},
			expectedNil: true,
		},
		{
			name:          "single param with two values",
			params:        map[string][]string{"env": {"dev", "prod"}},
			expectedCount: 2,
		},
		{
			name: "two params cartesian product",
			params: map[string][]string{
				"env":     {"dev", "prod"},
				"version": {"1.0", "2.0", "3.0"},
			},
			expectedCount: 6,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GenerateParameterCombinations(tt.params)
			if tt.expectedNil {
				assert.Nil(t, result)
			} else {
				assert.Len(t, result, tt.expectedCount)
				// Verify each combination has all keys
				for _, combo := range result {
					for key := range tt.params {
						assert.Contains(t, combo, key)
					}
				}
			}
		})
	}
}

func TestExtractJSONPath(t *testing.T) {
	tests := []struct {
		name      string
		jsonStr   string
		path      string
		expected  string
		expectErr bool
	}{
		{
			name:     "simple field",
			jsonStr:  `{"name": "hello"}`,
			path:     "$.name",
			expected: "hello",
		},
		{
			name:     "nested field",
			jsonStr:  `{"data": {"value": "nested"}}`,
			path:     "$.data.value",
			expected: "nested",
		},
		{
			name:     "array access",
			jsonStr:  `{"items": ["a", "b", "c"]}`,
			path:     "$.items[1]",
			expected: "b",
		},
		{
			name:      "missing field",
			jsonStr:   `{"name": "hello"}`,
			path:      "$.missing",
			expectErr: true,
		},
		{
			name:     "numeric value",
			jsonStr:  `{"count": 42}`,
			path:     "$.count",
			expected: "42",
		},
		{
			name:      "invalid JSON",
			jsonStr:   `not json`,
			path:      "$.field",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ExtractJSONPath(tt.jsonStr, tt.path)
			if tt.expectErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestExtractRegex(t *testing.T) {
	tests := []struct {
		name      string
		text      string
		pattern   string
		expected  string
		expectErr bool
	}{
		{
			name:     "full match",
			text:     "version: 1.2.3",
			pattern:  `\d+\.\d+\.\d+`,
			expected: "1.2.3",
		},
		{
			name:     "group capture",
			text:     "version: 1.2.3",
			pattern:  `version: (\d+\.\d+\.\d+)`,
			expected: "1.2.3",
		},
		{
			name:      "no match",
			text:      "hello world",
			pattern:   `\d+`,
			expectErr: true,
		},
		{
			name:      "invalid pattern",
			text:      "test",
			pattern:   `[invalid`,
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ExtractRegex(tt.text, tt.pattern)
			if tt.expectErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestReadFile(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(t *testing.T) string
		expectErr bool
		expected  string
	}{
		{
			name: "existing file",
			setup: func(t *testing.T) string {
				t.Helper()
				dir := t.TempDir()
				path := filepath.Join(dir, "test.txt")
				err := os.WriteFile(path, []byte("file content"), 0o644)
				require.NoError(t, err)
				return path
			},
			expected: "file content",
		},
		{
			name: "non-existent file",
			setup: func(t *testing.T) string {
				t.Helper()
				return filepath.Join(t.TempDir(), "nonexistent.txt")
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := tt.setup(t)
			result, err := ReadFile(path)
			if tt.expectErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestDefaultActivityOptions(t *testing.T) {
	opts := DefaultActivityOptions()
	assert.Equal(t, 10*60_000_000_000, int(opts.StartToCloseTimeout.Nanoseconds()))
	require.NotNil(t, opts.RetryPolicy)
	assert.Equal(t, int32(3), opts.RetryPolicy.MaximumAttempts)
	assert.Equal(t, 2.0, opts.RetryPolicy.BackoffCoefficient)
}

func TestFailureStrategyConstants(t *testing.T) {
	assert.Equal(t, "fail_fast", FailureStrategyFailFast)
	assert.Equal(t, "continue", FailureStrategyContinue)
}
