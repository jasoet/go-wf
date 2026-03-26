package workflow

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jasoet/go-wf/container/payload"
	generic "github.com/jasoet/go-wf/workflow"
)

func TestExtractOutput_Stdout(t *testing.T) {
	output := &payload.ContainerExecutionOutput{
		Stdout:   "version: v1.2.3",
		Stderr:   "",
		ExitCode: 0,
		Success:  true,
	}

	tests := []struct {
		name    string
		def     payload.OutputDefinition
		want    string
		wantErr bool
	}{
		{
			name: "extract from stdout",
			def: payload.OutputDefinition{
				Name:      "version",
				ValueFrom: "stdout",
			},
			want:    "version: v1.2.3",
			wantErr: false,
		},
		{
			name: "extract with regex",
			def: payload.OutputDefinition{
				Name:      "version",
				ValueFrom: "stdout",
				Regex:     `v(\d+\.\d+\.\d+)`,
			},
			want:    "1.2.3",
			wantErr: false,
		},
		{
			name: "extract with default on regex mismatch",
			def: payload.OutputDefinition{
				Name:      "version",
				ValueFrom: "stdout",
				Regex:     `build: (\d+)`,
				Default:   "unknown",
			},
			want:    "unknown",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ExtractOutput(tt.def, output)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestExtractOutput_ExitCode(t *testing.T) {
	output := &payload.ContainerExecutionOutput{
		Stdout:   "",
		Stderr:   "",
		ExitCode: 42,
		Success:  false,
	}

	def := payload.OutputDefinition{
		Name:      "exit_code",
		ValueFrom: "exitCode",
	}

	got, err := ExtractOutput(def, output)
	require.NoError(t, err)
	assert.Equal(t, "42", got)
}

func TestExtractOutput_Stderr(t *testing.T) {
	output := &payload.ContainerExecutionOutput{
		Stdout:   "",
		Stderr:   "Error: something went wrong",
		ExitCode: 1,
		Success:  false,
	}

	def := payload.OutputDefinition{
		Name:      "error",
		ValueFrom: "stderr",
	}

	got, err := ExtractOutput(def, output)
	require.NoError(t, err)
	assert.Equal(t, "Error: something went wrong", got)
}

func TestExtractOutput_FileSuccess(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "output.txt")
	err := os.WriteFile(filePath, []byte("file-content-123"), 0o644)
	require.NoError(t, err)

	def := payload.OutputDefinition{
		Name:      "file_output",
		ValueFrom: "file",
		Path:      filePath,
	}

	output := &payload.ContainerExecutionOutput{}

	got, err := ExtractOutput(def, output)
	require.NoError(t, err)
	assert.Equal(t, "file-content-123", got)
}

func TestExtractOutput_FileErrorWithDefault(t *testing.T) {
	def := payload.OutputDefinition{
		Name:      "file_output",
		ValueFrom: "file",
		Path:      "/nonexistent/path/file.txt",
		Default:   "fallback-value",
	}

	output := &payload.ContainerExecutionOutput{}

	got, err := ExtractOutput(def, output)
	require.NoError(t, err)
	assert.Equal(t, "fallback-value", got)
}

func TestExtractOutput_FileErrorNoDefault(t *testing.T) {
	def := payload.OutputDefinition{
		Name:      "file_output",
		ValueFrom: "file",
		Path:      "/nonexistent/path/file.txt",
	}

	output := &payload.ContainerExecutionOutput{}

	_, err := ExtractOutput(def, output)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read file")
}

func TestExtractOutput_FileMissingPath(t *testing.T) {
	def := payload.OutputDefinition{
		Name:      "file_output",
		ValueFrom: "file",
		Path:      "",
	}

	output := &payload.ContainerExecutionOutput{}

	_, err := ExtractOutput(def, output)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "path is required")
}

func TestExtractOutput_UnknownValueFrom(t *testing.T) {
	def := payload.OutputDefinition{
		Name:      "unknown_output",
		ValueFrom: "unknown_source",
		Default:   "default-val",
	}

	output := &payload.ContainerExecutionOutput{}

	got, err := ExtractOutput(def, output)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown value_from")
	assert.Equal(t, "default-val", got)
}

func TestExtractOutput_EmptyStdoutWithDefault(t *testing.T) {
	output := &payload.ContainerExecutionOutput{
		Stdout: "",
	}

	def := payload.OutputDefinition{
		Name:      "empty_stdout",
		ValueFrom: "stdout",
		Default:   "fallback",
	}

	got, err := ExtractOutput(def, output)
	require.NoError(t, err)
	assert.Equal(t, "fallback", got)
}

func TestExtractOutput_WhitespaceTrimming(t *testing.T) {
	output := &payload.ContainerExecutionOutput{
		Stdout: "  hello world  \n",
	}

	def := payload.OutputDefinition{
		Name:      "trimmed",
		ValueFrom: "stdout",
	}

	got, err := ExtractOutput(def, output)
	require.NoError(t, err)
	assert.Equal(t, "hello world", got)
}

func TestExtractOutput_JSONPathNull(t *testing.T) {
	output := &payload.ContainerExecutionOutput{
		Stdout: `{"field": null}`,
	}

	def := payload.OutputDefinition{
		Name:      "null_field",
		ValueFrom: "stdout",
		JSONPath:  "$.field",
	}

	got, err := ExtractOutput(def, output)
	require.NoError(t, err)
	assert.Equal(t, "", got)
}

func TestExtractOutput_JSONPathFailureWithDefault(t *testing.T) {
	output := &payload.ContainerExecutionOutput{
		Stdout: `{"version": "1.2.3"}`,
	}

	def := payload.OutputDefinition{
		Name:      "missing_field",
		ValueFrom: "stdout",
		JSONPath:  "$.missing_field",
		Default:   "default-version",
	}

	got, err := ExtractOutput(def, output)
	require.NoError(t, err)
	assert.Equal(t, "default-version", got)
}

func TestExtractJSONPath(t *testing.T) {
	tests := []struct {
		name    string
		jsonStr string
		path    string
		want    string
		wantErr bool
	}{
		{
			name:    "simple field",
			jsonStr: `{"version": "1.2.3"}`,
			path:    "$.version",
			want:    "1.2.3",
			wantErr: false,
		},
		{
			name:    "nested field",
			jsonStr: `{"build": {"id": "12345", "version": "1.2.3"}}`,
			path:    "$.build.id",
			want:    "12345",
			wantErr: false,
		},
		{
			name:    "array index",
			jsonStr: `{"tags": ["v1", "v2", "v3"]}`,
			path:    "$.tags[1]",
			want:    "v2",
			wantErr: false,
		},
		{
			name:    "nested array",
			jsonStr: `{"builds": [{"id": "1"}, {"id": "2"}]}`,
			path:    "$.builds[0].id",
			want:    "1",
			wantErr: false,
		},
		{
			name:    "number field",
			jsonStr: `{"count": 42}`,
			path:    "$.count",
			want:    "42",
			wantErr: false,
		},
		{
			name:    "boolean field",
			jsonStr: `{"success": true}`,
			path:    "$.success",
			want:    "true",
			wantErr: false,
		},
		{
			name:    "missing field",
			jsonStr: `{"version": "1.2.3"}`,
			path:    "$.missing",
			want:    "",
			wantErr: true,
		},
		{
			name:    "invalid JSON",
			jsonStr: `{invalid}`,
			path:    "$.version",
			want:    "",
			wantErr: true,
		},
		{
			name:    "path without $",
			jsonStr: `{"version": "1.2.3"}`,
			path:    "version",
			want:    "1.2.3",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := generic.ExtractJSONPath(tt.jsonStr, tt.path)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestExtractRegex(t *testing.T) {
	tests := []struct {
		name    string
		text    string
		pattern string
		want    string
		wantErr bool
	}{
		{
			name:    "simple match",
			text:    "version: v1.2.3",
			pattern: `v\d+\.\d+\.\d+`,
			want:    "v1.2.3",
			wantErr: false,
		},
		{
			name:    "capturing group",
			text:    "version: v1.2.3",
			pattern: `v(\d+\.\d+\.\d+)`,
			want:    "1.2.3",
			wantErr: false,
		},
		{
			name:    "multiple capturing groups",
			text:    "build: 12345, version: v1.2.3",
			pattern: `build: (\d+)`,
			want:    "12345",
			wantErr: false,
		},
		{
			name:    "no match",
			text:    "version: v1.2.3",
			pattern: `build: (\d+)`,
			want:    "",
			wantErr: true,
		},
		{
			name:    "invalid regex",
			text:    "version: v1.2.3",
			pattern: `[invalid`,
			want:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := generic.ExtractRegex(tt.text, tt.pattern)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestExtractOutputs(t *testing.T) {
	output := &payload.ContainerExecutionOutput{
		Stdout:   `{"build": {"id": "12345", "version": "1.2.3"}}`,
		Stderr:   "",
		ExitCode: 0,
		Success:  true,
	}

	definitions := []payload.OutputDefinition{
		{
			Name:      "build_id",
			ValueFrom: "stdout",
			JSONPath:  "$.build.id",
		},
		{
			Name:      "version",
			ValueFrom: "stdout",
			JSONPath:  "$.build.version",
		},
		{
			Name:      "exit_code",
			ValueFrom: "exitCode",
		},
	}

	got, err := ExtractOutputs(definitions, output)
	require.NoError(t, err)
	require.Len(t, got, 3)
	assert.Equal(t, "12345", got["build_id"])
	assert.Equal(t, "1.2.3", got["version"])
	assert.Equal(t, "0", got["exit_code"])
}

func TestSubstituteInputs(t *testing.T) {
	stepOutputs := map[string]map[string]string{
		"build": {
			"version":  "1.2.3",
			"build_id": "12345",
		},
		"test": {
			"result": "passed",
		},
	}

	containerInput := &payload.ContainerExecutionInput{
		Image: "deployer:v1",
	}

	inputs := []payload.InputMapping{
		{
			Name:     "BUILD_VERSION",
			From:     "build.version",
			Required: true,
		},
		{
			Name:     "BUILD_ID",
			From:     "build.build_id",
			Required: true,
		},
		{
			Name:     "TEST_RESULT",
			From:     "test.result",
			Required: false,
		},
		{
			Name:     "DEPLOY_ENV",
			From:     "missing.value",
			Required: false,
			Default:  "staging",
		},
	}

	err := SubstituteInputs(containerInput, inputs, stepOutputs)
	require.NoError(t, err)
	assert.Equal(t, "1.2.3", containerInput.Env["BUILD_VERSION"])
	assert.Equal(t, "12345", containerInput.Env["BUILD_ID"])
	assert.Equal(t, "passed", containerInput.Env["TEST_RESULT"])
	assert.Equal(t, "staging", containerInput.Env["DEPLOY_ENV"])
}

func TestSubstituteInputs_RequiredMissing(t *testing.T) {
	stepOutputs := map[string]map[string]string{
		"build": {
			"version": "1.2.3",
		},
	}

	containerInput := &payload.ContainerExecutionInput{
		Image: "deployer:v1",
	}

	inputs := []payload.InputMapping{
		{
			Name:     "MISSING_VALUE",
			From:     "missing.value",
			Required: true,
		},
	}

	err := SubstituteInputs(containerInput, inputs, stepOutputs)
	require.Error(t, err)
}

func TestResolveInputMapping(t *testing.T) {
	stepOutputs := map[string]map[string]string{
		"build": {
			"version":  "1.2.3",
			"build_id": "12345",
		},
	}

	tests := []struct {
		name    string
		mapping payload.InputMapping
		want    string
		wantErr bool
	}{
		{
			name: "valid mapping",
			mapping: payload.InputMapping{
				Name: "VERSION",
				From: "build.version",
			},
			want:    "1.2.3",
			wantErr: false,
		},
		{
			name: "missing step",
			mapping: payload.InputMapping{
				Name:    "VERSION",
				From:    "missing.version",
				Default: "default",
			},
			want:    "default",
			wantErr: true,
		},
		{
			name: "missing output",
			mapping: payload.InputMapping{
				Name:    "VALUE",
				From:    "build.missing",
				Default: "default",
			},
			want:    "default",
			wantErr: true,
		},
		{
			name: "invalid format",
			mapping: payload.InputMapping{
				Name:    "VALUE",
				From:    "invalid",
				Default: "",
			},
			want:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveInputMapping(tt.mapping, stepOutputs)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

// Benchmark tests.
func BenchmarkExtractJSONPath(b *testing.B) {
	jsonStr := `{"build": {"id": "12345", "version": "1.2.3", "tags": ["v1", "v2", "v3"]}}`
	path := "$.build.id"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = generic.ExtractJSONPath(jsonStr, path)
	}
}

func BenchmarkExtractRegex(b *testing.B) {
	text := "version: v1.2.3, build: 12345"
	pattern := `v(\d+\.\d+\.\d+)`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = generic.ExtractRegex(text, pattern)
	}
}

func BenchmarkExtractOutputs(b *testing.B) {
	output := &payload.ContainerExecutionOutput{
		Stdout:   `{"build": {"id": "12345", "version": "1.2.3"}}`,
		ExitCode: 0,
		Success:  true,
	}

	definitions := []payload.OutputDefinition{
		{Name: "build_id", ValueFrom: "stdout", JSONPath: "$.build.id"},
		{Name: "version", ValueFrom: "stdout", JSONPath: "$.build.version"},
		{Name: "exit_code", ValueFrom: "exitCode"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ExtractOutputs(definitions, output)
	}
}
