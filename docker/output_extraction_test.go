package docker

import (
	"testing"
)

func TestExtractOutput_Stdout(t *testing.T) {
	output := &ContainerExecutionOutput{
		Stdout:   "version: v1.2.3",
		Stderr:   "",
		ExitCode: 0,
		Success:  true,
	}

	tests := []struct {
		name    string
		def     OutputDefinition
		want    string
		wantErr bool
	}{
		{
			name: "extract from stdout",
			def: OutputDefinition{
				Name:      "version",
				ValueFrom: "stdout",
			},
			want:    "version: v1.2.3",
			wantErr: false,
		},
		{
			name: "extract with regex",
			def: OutputDefinition{
				Name:      "version",
				ValueFrom: "stdout",
				Regex:     `v(\d+\.\d+\.\d+)`,
			},
			want:    "1.2.3",
			wantErr: false,
		},
		{
			name: "extract with default on regex mismatch",
			def: OutputDefinition{
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
			if (err != nil) != tt.wantErr {
				t.Errorf("ExtractOutput() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ExtractOutput() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExtractOutput_ExitCode(t *testing.T) {
	output := &ContainerExecutionOutput{
		Stdout:   "",
		Stderr:   "",
		ExitCode: 42,
		Success:  false,
	}

	def := OutputDefinition{
		Name:      "exit_code",
		ValueFrom: "exitCode",
	}

	got, err := ExtractOutput(def, output)
	if err != nil {
		t.Errorf("ExtractOutput() error = %v", err)
		return
	}
	if got != "42" {
		t.Errorf("ExtractOutput() = %v, want 42", got)
	}
}

func TestExtractOutput_Stderr(t *testing.T) {
	output := &ContainerExecutionOutput{
		Stdout:   "",
		Stderr:   "Error: something went wrong",
		ExitCode: 1,
		Success:  false,
	}

	def := OutputDefinition{
		Name:      "error",
		ValueFrom: "stderr",
	}

	got, err := ExtractOutput(def, output)
	if err != nil {
		t.Errorf("ExtractOutput() error = %v", err)
		return
	}
	if got != "Error: something went wrong" {
		t.Errorf("ExtractOutput() = %v, want error message", got)
	}
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
			got, err := extractJSONPath(tt.jsonStr, tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("extractJSONPath() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("extractJSONPath() = %v, want %v", got, tt.want)
			}
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
			got, err := extractRegex(tt.text, tt.pattern)
			if (err != nil) != tt.wantErr {
				t.Errorf("extractRegex() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("extractRegex() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExtractOutputs(t *testing.T) {
	output := &ContainerExecutionOutput{
		Stdout:   `{"build": {"id": "12345", "version": "1.2.3"}}`,
		Stderr:   "",
		ExitCode: 0,
		Success:  true,
	}

	definitions := []OutputDefinition{
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
	if err != nil {
		t.Errorf("ExtractOutputs() error = %v", err)
		return
	}

	if len(got) != 3 {
		t.Errorf("ExtractOutputs() returned %d outputs, want 3", len(got))
	}

	if got["build_id"] != "12345" {
		t.Errorf("build_id = %v, want 12345", got["build_id"])
	}

	if got["version"] != "1.2.3" {
		t.Errorf("version = %v, want 1.2.3", got["version"])
	}

	if got["exit_code"] != "0" {
		t.Errorf("exit_code = %v, want 0", got["exit_code"])
	}
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

	containerInput := &ContainerExecutionInput{
		Image: "deployer:v1",
	}

	inputs := []InputMapping{
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
	if err != nil {
		t.Errorf("SubstituteInputs() error = %v", err)
		return
	}

	if containerInput.Env["BUILD_VERSION"] != "1.2.3" {
		t.Errorf("BUILD_VERSION = %v, want 1.2.3", containerInput.Env["BUILD_VERSION"])
	}

	if containerInput.Env["BUILD_ID"] != "12345" {
		t.Errorf("BUILD_ID = %v, want 12345", containerInput.Env["BUILD_ID"])
	}

	if containerInput.Env["TEST_RESULT"] != "passed" {
		t.Errorf("TEST_RESULT = %v, want passed", containerInput.Env["TEST_RESULT"])
	}

	if containerInput.Env["DEPLOY_ENV"] != "staging" {
		t.Errorf("DEPLOY_ENV = %v, want staging", containerInput.Env["DEPLOY_ENV"])
	}
}

func TestSubstituteInputs_RequiredMissing(t *testing.T) {
	stepOutputs := map[string]map[string]string{
		"build": {
			"version": "1.2.3",
		},
	}

	containerInput := &ContainerExecutionInput{
		Image: "deployer:v1",
	}

	inputs := []InputMapping{
		{
			Name:     "MISSING_VALUE",
			From:     "missing.value",
			Required: true,
		},
	}

	err := SubstituteInputs(containerInput, inputs, stepOutputs)
	if err == nil {
		t.Error("SubstituteInputs() should return error for missing required input")
	}
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
		mapping InputMapping
		want    string
		wantErr bool
	}{
		{
			name: "valid mapping",
			mapping: InputMapping{
				Name: "VERSION",
				From: "build.version",
			},
			want:    "1.2.3",
			wantErr: false,
		},
		{
			name: "missing step",
			mapping: InputMapping{
				Name:    "VERSION",
				From:    "missing.version",
				Default: "default",
			},
			want:    "default",
			wantErr: true,
		},
		{
			name: "missing output",
			mapping: InputMapping{
				Name:    "VALUE",
				From:    "build.missing",
				Default: "default",
			},
			want:    "default",
			wantErr: true,
		},
		{
			name: "invalid format",
			mapping: InputMapping{
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
			if (err != nil) != tt.wantErr {
				t.Errorf("resolveInputMapping() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("resolveInputMapping() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Benchmark tests
func BenchmarkExtractJSONPath(b *testing.B) {
	jsonStr := `{"build": {"id": "12345", "version": "1.2.3", "tags": ["v1", "v2", "v3"]}}`
	path := "$.build.id"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = extractJSONPath(jsonStr, path)
	}
}

func BenchmarkExtractRegex(b *testing.B) {
	text := "version: v1.2.3, build: 12345"
	pattern := `v(\d+\.\d+\.\d+)`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = extractRegex(text, pattern)
	}
}

func BenchmarkExtractOutputs(b *testing.B) {
	output := &ContainerExecutionOutput{
		Stdout:   `{"build": {"id": "12345", "version": "1.2.3"}}`,
		ExitCode: 0,
		Success:  true,
	}

	definitions := []OutputDefinition{
		{Name: "build_id", ValueFrom: "stdout", JSONPath: "$.build.id"},
		{Name: "version", ValueFrom: "stdout", JSONPath: "$.build.version"},
		{Name: "exit_code", ValueFrom: "exitCode"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ExtractOutputs(definitions, output)
	}
}
