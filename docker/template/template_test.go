package template

import (
	"testing"

	"github.com/jasoet/go-wf/docker/payload"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewContainer(t *testing.T) {
	container := NewContainer("deploy", "myapp:v1",
		WithCommand("deploy.sh"),
		WithEnv("ENV", "production"),
		WithPorts("8080:8080"),
		WithAutoRemove(true))

	input := container.ToInput()
	assert.Equal(t, "myapp:v1", input.Image)
	assert.Equal(t, []string{"deploy.sh"}, input.Command)
	assert.Equal(t, "production", input.Env["ENV"])
	assert.Contains(t, input.Ports, "8080:8080")
	assert.True(t, input.AutoRemove)
}

func TestNewScript(t *testing.T) {
	tests := []struct {
		name      string
		language  string
		script    string
		wantImage string
	}{
		{"bash script", "bash", "echo 'test'", "bash:5.2"},
		{"python script", "python", "print('test')", "python:3.11-slim"},
		{"node script", "node", "console.log('test')", "node:20-slim"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			script := NewScript("test", tt.language,
				WithScriptContent(tt.script))

			input := script.ToInput()
			assert.Equal(t, tt.wantImage, input.Image)
			assert.Contains(t, input.Command, tt.script)
		})
	}
}

func TestNewBashScript(t *testing.T) {
	script := NewBashScript("backup",
		"tar -czf backup.tar.gz /data",
		WithScriptEnv("BACKUP_DIR", "/backup"))

	input := script.ToInput()
	assert.Equal(t, "bash:5.2", input.Image)
	assert.Contains(t, input.Command, "tar -czf backup.tar.gz /data")
	assert.Equal(t, "/backup", input.Env["BACKUP_DIR"])
}

func TestNewHTTP(t *testing.T) {
	http := NewHTTP("health-check",
		WithHTTPURL("https://api.example.com/health"),
		WithHTTPMethod("GET"),
		WithHTTPExpectedStatus(200))

	require.NoError(t, http.Validate())

	input := http.ToInput()
	assert.Equal(t, "curlimages/curl:latest", input.Image)
	assert.NotEmpty(t, input.Command)
}

func TestNewHTTPHealthCheck(t *testing.T) {
	healthCheck := NewHTTPHealthCheck("api-health", "https://api.example.com/health")

	input := healthCheck.ToInput()
	assert.Equal(t, "curlimages/curl:latest", input.Image)
	assert.Equal(t, "api-health", input.Name)
}

func TestNewHTTPWebhook(t *testing.T) {
	webhook := NewHTTPWebhook("slack-notify",
		"https://hooks.slack.com/services/...",
		`{"text": "Deployment complete"}`)

	input := webhook.ToInput()
	assert.Equal(t, "curlimages/curl:latest", input.Image)
	assert.Equal(t, "slack-notify", input.Name)
}

func TestContainerWaitStrategies(t *testing.T) {
	tests := []struct {
		name     string
		option   ContainerOption
		validate func(t *testing.T, input payload.ContainerExecutionInput)
	}{
		{
			name:   "wait for log",
			option: WithWaitForLog("ready to accept connections"),
			validate: func(t *testing.T, input payload.ContainerExecutionInput) {
				assert.Equal(t, "log", input.WaitStrategy.Type)
				assert.Equal(t, "ready to accept connections", input.WaitStrategy.LogMessage)
			},
		},
		{
			name:   "wait for port",
			option: WithWaitForPort("5432"),
			validate: func(t *testing.T, input payload.ContainerExecutionInput) {
				assert.Equal(t, "port", input.WaitStrategy.Type)
				assert.Equal(t, "5432", input.WaitStrategy.Port)
			},
		},
		{
			name:   "wait for HTTP",
			option: WithWaitForHTTP("8080", "/health", 200),
			validate: func(t *testing.T, input payload.ContainerExecutionInput) {
				assert.Equal(t, "http", input.WaitStrategy.Type)
				assert.Equal(t, "8080", input.WaitStrategy.Port)
				assert.Equal(t, "/health", input.WaitStrategy.HTTPPath)
				assert.Equal(t, 200, input.WaitStrategy.HTTPStatus)
			},
		},
		{
			name:   "wait for healthy",
			option: WithWaitForHealthy(),
			validate: func(t *testing.T, input payload.ContainerExecutionInput) {
				assert.Equal(t, "healthy", input.WaitStrategy.Type)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			container := NewContainer("test", "alpine:latest", tt.option)
			input := container.ToInput()
			tt.validate(t, input)
		})
	}
}

func TestScriptValidation(t *testing.T) {
	tests := []struct {
		name        string
		script      *Script
		expectError bool
	}{
		{
			name:        "valid script",
			script:      NewBashScript("test", "echo 'hello'"),
			expectError: false,
		},
		{
			name: "missing name",
			script: &Script{
				language: "bash",
				image:    "bash:5.2",
				command:  []string{"bash", "-c"},
			},
			expectError: true,
		},
		{
			name: "missing image",
			script: &Script{
				name:     "test",
				language: "bash",
				command:  []string{"bash", "-c"},
			},
			expectError: true,
		},
		{
			name: "missing command",
			script: &Script{
				name:     "test",
				language: "bash",
				image:    "bash:5.2",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.script.Validate()
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestHTTPValidation(t *testing.T) {
	tests := []struct {
		name        string
		http        *HTTP
		expectError bool
	}{
		{
			name:        "valid HTTP",
			http:        NewHTTP("test", WithHTTPURL("https://example.com")),
			expectError: false,
		},
		{
			name:        "missing URL",
			http:        NewHTTP("test"),
			expectError: true,
		},
		{
			name: "missing name",
			http: &HTTP{
				url:       "https://example.com",
				method:    "GET",
				curlImage: "curlimages/curl:latest",
			},
			expectError: true,
		},
		{
			name: "missing method",
			http: &HTTP{
				name: "test",
				url:  "https://example.com",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.http.Validate()
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestScriptOptionsComprehensive(t *testing.T) {
	tests := []struct {
		name     string
		script   *Script
		expected func(*testing.T, payload.ContainerExecutionInput)
	}{
		{
			name: "script with custom image",
			script: NewScript("test", "bash",
				WithScriptImage("custom/bash:v1"),
				WithScriptContent("echo test")),
			expected: func(t *testing.T, input payload.ContainerExecutionInput) {
				assert.Equal(t, "custom/bash:v1", input.Image)
			},
		},
		{
			name: "script with environment variables",
			script: NewBashScript("test", "echo $VAR1 $VAR2",
				WithScriptEnv("VAR1", "value1"),
				WithScriptEnv("VAR2", "value2")),
			expected: func(t *testing.T, input payload.ContainerExecutionInput) {
				assert.Equal(t, "value1", input.Env["VAR1"])
				assert.Equal(t, "value2", input.Env["VAR2"])
			},
		},
		{
			name: "script with environment map",
			script: NewBashScript("test", "echo $VAR1 $VAR2 $VAR3",
				WithScriptEnvMap(map[string]string{
					"VAR1": "value1",
					"VAR2": "value2",
					"VAR3": "value3",
				})),
			expected: func(t *testing.T, input payload.ContainerExecutionInput) {
				assert.Equal(t, "value1", input.Env["VAR1"])
				assert.Equal(t, "value2", input.Env["VAR2"])
				assert.Equal(t, "value3", input.Env["VAR3"])
			},
		},
		{
			name: "script with work directory",
			script: NewPythonScript("test", "print('hello')",
				WithScriptWorkingDir("/workspace")),
			expected: func(t *testing.T, input payload.ContainerExecutionInput) {
				assert.Equal(t, "/workspace", input.WorkDir)
			},
		},
		{
			name: "script with volume",
			script: NewNodeScript("test", "console.log('test')",
				WithScriptVolume("/host", "/container")),
			expected: func(t *testing.T, input payload.ContainerExecutionInput) {
				assert.Equal(t, "/container", input.Volumes["/host"])
			},
		},
		{
			name: "script with auto remove",
			script: NewBashScript("test", "echo test",
				WithScriptAutoRemove(true)),
			expected: func(t *testing.T, input payload.ContainerExecutionInput) {
				assert.True(t, input.AutoRemove)
			},
		},
		{
			name: "script with ports",
			script: NewBashScript("test", "echo test",
				WithScriptPorts("8080:8080", "9090:9090")),
			expected: func(t *testing.T, input payload.ContainerExecutionInput) {
				assert.Contains(t, input.Ports, "8080:8080")
				assert.Contains(t, input.Ports, "9090:9090")
			},
		},
		{
			name: "script with custom command",
			script: NewScript("test", "bash",
				WithScriptCommand("bash", "-x", "-e")),
			expected: func(t *testing.T, input payload.ContainerExecutionInput) {
				assert.Contains(t, input.Command, "bash")
				assert.Contains(t, input.Command, "-x")
				assert.Contains(t, input.Command, "-e")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := tt.script.ToInput()
			tt.expected(t, input)
		})
	}
}

func TestScriptLanguageHelpers(t *testing.T) {
	tests := []struct {
		name          string
		createScript  func() *Script
		expectedImage string
	}{
		{
			name:          "ruby script",
			createScript:  func() *Script { return NewRubyScript("test", "puts 'hello'") },
			expectedImage: "ruby:3.2-slim",
		},
		{
			name:          "golang script",
			createScript:  func() *Script { return NewGoScript("test", "package main") },
			expectedImage: "golang:1.25",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			script := tt.createScript()
			input := script.ToInput()
			assert.Equal(t, tt.expectedImage, input.Image)
		})
	}
}

func TestHTTPOptionsComprehensive(t *testing.T) {
	tests := []struct {
		name     string
		http     *HTTP
		expected func(*testing.T, payload.ContainerExecutionInput)
	}{
		{
			name: "http with headers",
			http: NewHTTP("test",
				WithHTTPURL("https://api.example.com"),
				WithHTTPHeader("Authorization", "Bearer token"),
				WithHTTPHeader("Content-Type", "application/json")),
			expected: func(t *testing.T, input payload.ContainerExecutionInput) {
				script := input.Command[len(input.Command)-1]
				assert.Contains(t, script, "-H")
				assert.Contains(t, script, "Authorization: Bearer token")
				assert.Contains(t, script, "Content-Type: application/json")
			},
		},
		{
			name: "http with body",
			http: NewHTTP("test",
				WithHTTPURL("https://api.example.com"),
				WithHTTPMethod("POST"),
				WithHTTPBody(`{"key": "value"}`)),
			expected: func(t *testing.T, input payload.ContainerExecutionInput) {
				script := input.Command[len(input.Command)-1]
				assert.Contains(t, script, "-d")
			},
		},
		{
			name: "http with expected status",
			http: NewHTTP("test",
				WithHTTPURL("https://api.example.com"),
				WithHTTPExpectedStatus(201)),
			expected: func(t *testing.T, input payload.ContainerExecutionInput) {
				script := input.Command[len(input.Command)-1]
				assert.Contains(t, script, "201")
			},
		},
		{
			name: "http with timeout",
			http: NewHTTP("test",
				WithHTTPURL("https://api.example.com"),
				WithHTTPTimeout(45)),
			expected: func(t *testing.T, input payload.ContainerExecutionInput) {
				script := input.Command[len(input.Command)-1]
				assert.Contains(t, script, "--max-time 45")
			},
		},
		{
			name: "http with custom image",
			http: NewHTTP("test",
				WithHTTPURL("https://api.example.com"),
				WithHTTPCurlImage("custom/curl:v1")),
			expected: func(t *testing.T, input payload.ContainerExecutionInput) {
				assert.Equal(t, "custom/curl:v1", input.Image)
			},
		},
		{
			name: "http with multiple headers",
			http: NewHTTP("test",
				WithHTTPURL("https://api.example.com"),
				WithHTTPHeaders(map[string]string{
					"Authorization": "Bearer token",
					"Content-Type":  "application/json",
					"Accept":        "application/json",
				})),
			expected: func(t *testing.T, input payload.ContainerExecutionInput) {
				script := input.Command[len(input.Command)-1]
				assert.Contains(t, script, "Authorization: Bearer token")
				assert.Contains(t, script, "Content-Type: application/json")
				assert.Contains(t, script, "Accept: application/json")
			},
		},
		{
			name: "http with auto remove",
			http: NewHTTP("test",
				WithHTTPURL("https://api.example.com"),
				WithHTTPAutoRemove(true)),
			expected: func(t *testing.T, input payload.ContainerExecutionInput) {
				assert.True(t, input.AutoRemove)
			},
		},
		{
			name: "http with follow redirect",
			http: NewHTTP("test",
				WithHTTPURL("https://api.example.com"),
				WithHTTPFollowRedirect(true)),
			expected: func(t *testing.T, input payload.ContainerExecutionInput) {
				script := input.Command[len(input.Command)-1]
				assert.Contains(t, script, "-L")
			},
		},
		{
			name: "http with insecure",
			http: NewHTTP("test",
				WithHTTPURL("https://api.example.com"),
				WithHTTPInsecure(true)),
			expected: func(t *testing.T, input payload.ContainerExecutionInput) {
				script := input.Command[len(input.Command)-1]
				assert.Contains(t, script, "-k")
			},
		},
		{
			name: "http with env variables",
			http: NewHTTP("test",
				WithHTTPURL("https://api.example.com"),
				WithHTTPEnv("API_KEY", "secret123")),
			expected: func(t *testing.T, input payload.ContainerExecutionInput) {
				assert.Equal(t, "secret123", input.Env["API_KEY"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := tt.http.ToInput()
			tt.expected(t, input)
		})
	}
}

func TestContainerValidation(t *testing.T) {
	tests := []struct {
		name        string
		container   *Container
		expectError bool
	}{
		{
			name:        "valid container",
			container:   NewContainer("test", "alpine:latest", WithCommand("echo", "hello")),
			expectError: false,
		},
		{
			name: "missing name",
			container: &Container{
				image:   "alpine:latest",
				command: []string{"echo", "hello"},
			},
			expectError: true,
		},
		{
			name: "missing image",
			container: &Container{
				name:    "test",
				command: []string{"echo", "hello"},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.container.Validate()
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestContainerOptionsComprehensive(t *testing.T) {
	tests := []struct {
		name      string
		container *Container
		expected  func(*testing.T, payload.ContainerExecutionInput)
	}{
		{
			name: "container with entrypoint",
			container: NewContainer("test", "alpine:latest",
				WithEntrypoint("/bin/sh", "-c")),
			expected: func(t *testing.T, input payload.ContainerExecutionInput) {
				assert.Equal(t, []string{"/bin/sh", "-c"}, input.Entrypoint)
			},
		},
		{
			name: "container with labels",
			container: NewContainer("test", "alpine:latest",
				WithLabel("app", "myapp"),
				WithLabel("version", "1.0")),
			expected: func(t *testing.T, input payload.ContainerExecutionInput) {
				assert.Equal(t, "myapp", input.Labels["app"])
				assert.Equal(t, "1.0", input.Labels["version"])
			},
		},
		{
			name: "container with volumes",
			container: NewContainer("test", "alpine:latest",
				WithVolume("/host/path", "/container/path")),
			expected: func(t *testing.T, input payload.ContainerExecutionInput) {
				assert.Equal(t, "/container/path", input.Volumes["/host/path"])
			},
		},
		{
			name: "container with user",
			container: NewContainer("test", "alpine:latest",
				WithUser("1000:1000")),
			expected: func(t *testing.T, input payload.ContainerExecutionInput) {
				assert.Equal(t, "1000:1000", input.User)
			},
		},
		{
			name: "container with work dir",
			container: NewContainer("test", "alpine:latest",
				WithWorkDir("/workspace")),
			expected: func(t *testing.T, input payload.ContainerExecutionInput) {
				assert.Equal(t, "/workspace", input.WorkDir)
			},
		},
		{
			name: "container with env map",
			container: NewContainer("test", "alpine:latest",
				WithEnvMap(map[string]string{
					"VAR1": "value1",
					"VAR2": "value2",
				})),
			expected: func(t *testing.T, input payload.ContainerExecutionInput) {
				assert.Equal(t, "value1", input.Env["VAR1"])
				assert.Equal(t, "value2", input.Env["VAR2"])
			},
		},
		{
			name: "container with volumes map",
			container: NewContainer("test", "alpine:latest",
				WithVolumes(map[string]string{
					"/host1": "/container1",
					"/host2": "/container2",
				})),
			expected: func(t *testing.T, input payload.ContainerExecutionInput) {
				assert.Equal(t, "/container1", input.Volumes["/host1"])
				assert.Equal(t, "/container2", input.Volumes["/host2"])
			},
		},
		{
			name: "container with labels map",
			container: NewContainer("test", "alpine:latest",
				WithLabels(map[string]string{
					"app":     "myapp",
					"version": "1.0",
					"env":     "prod",
				})),
			expected: func(t *testing.T, input payload.ContainerExecutionInput) {
				assert.Equal(t, "myapp", input.Labels["app"])
				assert.Equal(t, "1.0", input.Labels["version"])
				assert.Equal(t, "prod", input.Labels["env"])
			},
		},
		{
			name: "container with wait strategy",
			container: NewContainer("test", "alpine:latest",
				WithWaitStrategy(payload.WaitStrategyConfig{
					Type:       "log",
					LogMessage: "ready",
				})),
			expected: func(t *testing.T, input payload.ContainerExecutionInput) {
				assert.Equal(t, "log", input.WaitStrategy.Type)
				assert.Equal(t, "ready", input.WaitStrategy.LogMessage)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := tt.container.ToInput()
			tt.expected(t, input)
		})
	}
}
