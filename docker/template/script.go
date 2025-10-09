package template

import (
	"fmt"

	"github.com/jasoet/go-wf/docker"
)

// Script is a WorkflowSource that creates a script-based container execution.
// It's useful for running inline scripts in various languages (bash, python, etc.).
//
// Example:
//
//	script := NewScript("backup", "bash",
//	    WithScriptContent(`
//	        echo "Creating backup..."
//	        tar -czf /backup/data.tar.gz /data
//	        echo "Backup complete"
//	    `))
type Script struct {
	name          string
	language      string
	image         string
	scriptContent string
	command       []string
	env           map[string]string
	workingDir    string
	autoRemove    bool
	volumes       map[string]string
	ports         []string
}

// NewScript creates a new script workflow source.
// The image should contain the interpreter for the script language.
//
// Parameters:
//   - name: Step name
//   - language: Script language ("bash", "python", "sh", etc.)
//   - opts: Optional configuration functions
//
// Example:
//
//	script := NewScript("process", "python",
//	    WithScriptContent("print('Processing data...')"),
//	    WithScriptEnv("API_KEY", "secret"))
func NewScript(name, language string, opts ...ScriptOption) *Script {
	s := &Script{
		name:       name,
		language:   language,
		env:        make(map[string]string),
		volumes:    make(map[string]string),
		ports:      make([]string, 0),
		autoRemove: true,
	}

	// Set default image and command based on language
	switch language {
	case "bash", "sh":
		s.image = "bash:5.2"
		s.command = []string{"bash", "-c"}
	case "python", "python3":
		s.image = "python:3.11-slim"
		s.command = []string{"python", "-c"}
	case "node", "nodejs", "javascript":
		s.image = "node:20-slim"
		s.command = []string{"node", "-e"}
	case "ruby":
		s.image = "ruby:3.2-slim"
		s.command = []string{"ruby", "-e"}
	case "go", "golang":
		s.image = "golang:1.25"
		s.command = []string{"go", "run", "-"}
	default:
		// Default to bash
		s.image = "bash:5.2"
		s.command = []string{"bash", "-c"}
	}

	// Apply options
	for _, opt := range opts {
		opt(s)
	}

	return s
}

// ToInput implements WorkflowSource interface.
func (s *Script) ToInput() docker.ContainerExecutionInput {
	// Validate script content
	if s.scriptContent == "" {
		// If no script content, create a no-op script
		s.scriptContent = "echo 'No script provided'"
	}

	// Build command with script content
	command := append(s.command, s.scriptContent)

	input := docker.ContainerExecutionInput{
		Image:      s.image,
		Command:    command,
		Env:        s.env,
		WorkDir:    s.workingDir,
		AutoRemove: s.autoRemove,
		Name:       s.name,
		Volumes:    s.volumes,
		Ports:      s.ports,
	}

	return input
}

// WithScriptContent sets the inline script content.
//
// Example:
//
//	script := NewScript("backup", "bash",
//	    WithScriptContent("tar -czf backup.tar.gz /data"))
func WithScriptContent(content string) ScriptOption {
	return func(s *Script) {
		s.scriptContent = content
	}
}

// WithScriptImage overrides the default image for the script.
//
// Example:
//
//	script := NewScript("custom", "python",
//	    WithScriptImage("custom/python:3.11"))
func WithScriptImage(image string) ScriptOption {
	return func(s *Script) {
		s.image = image
	}
}

// WithScriptCommand overrides the default command.
//
// Example:
//
//	script := NewScript("process", "python",
//	    WithScriptCommand("python3", "-u", "-c"))
func WithScriptCommand(cmd ...string) ScriptOption {
	return func(s *Script) {
		s.command = cmd
	}
}

// WithScriptEnv adds an environment variable.
//
// Example:
//
//	script := NewScript("deploy", "bash",
//	    WithScriptEnv("LOG_LEVEL", "debug"),
//	    WithScriptEnv("ENV", "production"))
func WithScriptEnv(name, value string) ScriptOption {
	return func(s *Script) {
		if s.env == nil {
			s.env = make(map[string]string)
		}
		s.env[name] = value
	}
}

// WithScriptEnvMap adds multiple environment variables.
//
// Example:
//
//	script := NewScript("deploy", "bash",
//	    WithScriptEnvMap(map[string]string{
//	        "LOG_LEVEL": "debug",
//	        "ENV": "production",
//	    }))
func WithScriptEnvMap(envMap map[string]string) ScriptOption {
	return func(s *Script) {
		if s.env == nil {
			s.env = make(map[string]string)
		}
		for k, v := range envMap {
			s.env[k] = v
		}
	}
}

// WithScriptWorkingDir sets the working directory.
//
// Example:
//
//	script := NewScript("build", "bash",
//	    WithScriptWorkingDir("/workspace"))
func WithScriptWorkingDir(dir string) ScriptOption {
	return func(s *Script) {
		s.workingDir = dir
	}
}

// WithScriptAutoRemove sets auto-remove behavior.
//
// Example:
//
//	script := NewScript("test", "bash",
//	    WithScriptAutoRemove(false))
func WithScriptAutoRemove(autoRemove bool) ScriptOption {
	return func(s *Script) {
		s.autoRemove = autoRemove
	}
}

// WithScriptVolume adds a volume mount.
//
// Example:
//
//	script := NewScript("backup", "bash",
//	    WithScriptVolume("/host/data", "/container/data"))
func WithScriptVolume(hostPath, containerPath string) ScriptOption {
	return func(s *Script) {
		if s.volumes == nil {
			s.volumes = make(map[string]string)
		}
		s.volumes[hostPath] = containerPath
	}
}

// WithScriptPorts adds port mappings.
//
// Example:
//
//	script := NewScript("server", "python",
//	    WithScriptPorts("8080:8080"))
func WithScriptPorts(ports ...string) ScriptOption {
	return func(s *Script) {
		s.ports = append(s.ports, ports...)
	}
}

// ScriptOption is a functional option for configuring Script.
type ScriptOption func(*Script)

// NewBashScript creates a bash script source with convenience methods.
//
// Example:
//
//	script := NewBashScript("backup",
//	    "tar -czf backup.tar.gz /data")
func NewBashScript(name, script string, opts ...ScriptOption) *Script {
	allOpts := append([]ScriptOption{WithScriptContent(script)}, opts...)
	return NewScript(name, "bash", allOpts...)
}

// NewPythonScript creates a Python script source with convenience methods.
//
// Example:
//
//	script := NewPythonScript("process",
//	    "import sys; print('Processing...')")
func NewPythonScript(name, script string, opts ...ScriptOption) *Script {
	allOpts := append([]ScriptOption{WithScriptContent(script)}, opts...)
	return NewScript(name, "python", allOpts...)
}

// NewNodeScript creates a new Node.js script template.
func NewNodeScript(name, script string, opts ...ScriptOption) *Script {
	allOpts := append([]ScriptOption{WithScriptContent(script)}, opts...)
	return NewScript(name, "node", allOpts...)
}

// NewRubyScript creates a new Ruby script template.
func NewRubyScript(name, script string, opts ...ScriptOption) *Script {
	allOpts := append([]ScriptOption{WithScriptContent(script)}, opts...)
	return NewScript(name, "ruby", allOpts...)
}

// NewGoScript creates a new Golang script template.
func NewGoScript(name, script string, opts ...ScriptOption) *Script {
	allOpts := append([]ScriptOption{WithScriptContent(script)}, opts...)
	return NewScript(name, "golang", allOpts...)
}

// Validate validates the script configuration.
func (s *Script) Validate() error {
	if s.name == "" {
		return fmt.Errorf("script name is required")
	}
	if s.image == "" {
		return fmt.Errorf("script image is required")
	}
	if len(s.command) == 0 {
		return fmt.Errorf("script command is required")
	}
	return nil
}
