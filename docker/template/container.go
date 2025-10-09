package template

import (
	"fmt"

	"github.com/jasoet/go-wf/docker"
)

// Container is a WorkflowSource that creates a container execution.
// It provides a fluent API for configuring containers with enhanced options.
//
// Example:
//
//	container := NewContainer("deploy", "myapp:v1",
//	    WithCommand("deploy.sh"),
//	    WithEnv("ENV", "production"),
//	    WithPorts("8080:8080"))
type Container struct {
	name         string
	image        string
	command      []string
	entrypoint   []string
	env          map[string]string
	ports        []string
	volumes      map[string]string
	workDir      string
	user         string
	autoRemove   bool
	labels       map[string]string
	waitStrategy docker.WaitStrategyConfig
}

// NewContainer creates a new container workflow source.
//
// Parameters:
//   - name: Container name
//   - image: Docker image (e.g., "alpine:latest", "myapp:v1")
//   - opts: Optional configuration functions
//
// Example:
//
//	container := NewContainer("deploy", "myapp:v1",
//	    WithCommand("deploy.sh"),
//	    WithEnv("ENV", "production"))
func NewContainer(name, image string, opts ...ContainerOption) *Container {
	c := &Container{
		name:       name,
		image:      image,
		env:        make(map[string]string),
		ports:      make([]string, 0),
		volumes:    make(map[string]string),
		labels:     make(map[string]string),
		autoRemove: true,
	}

	// Apply options
	for _, opt := range opts {
		opt(c)
	}

	return c
}

// ToInput implements WorkflowSource interface.
func (c *Container) ToInput() docker.ContainerExecutionInput {
	input := docker.ContainerExecutionInput{
		Image:        c.image,
		Command:      c.command,
		Entrypoint:   c.entrypoint,
		Env:          c.env,
		Ports:        c.ports,
		Volumes:      c.volumes,
		WorkDir:      c.workDir,
		User:         c.user,
		AutoRemove:   c.autoRemove,
		Name:         c.name,
		Labels:       c.labels,
		WaitStrategy: c.waitStrategy,
	}

	return input
}

// ContainerOption is a functional option for configuring Container.
type ContainerOption func(*Container)

// WithCommand sets the container command.
//
// Example:
//
//	container := NewContainer("build", "golang:1.25",
//	    WithCommand("go", "build", "-o", "app"))
func WithCommand(cmd ...string) ContainerOption {
	return func(c *Container) {
		c.command = cmd
	}
}

// WithEntrypoint sets the container entrypoint.
//
// Example:
//
//	container := NewContainer("process", "myapp:v1",
//	    WithEntrypoint("/bin/sh", "-c"))
func WithEntrypoint(entrypoint ...string) ContainerOption {
	return func(c *Container) {
		c.entrypoint = entrypoint
	}
}

// WithEnv adds an environment variable.
//
// Example:
//
//	container := NewContainer("deploy", "myapp:v1",
//	    WithEnv("LOG_LEVEL", "debug"),
//	    WithEnv("ENV", "production"))
func WithEnv(name, value string) ContainerOption {
	return func(c *Container) {
		if c.env == nil {
			c.env = make(map[string]string)
		}
		c.env[name] = value
	}
}

// WithEnvMap adds multiple environment variables.
//
// Example:
//
//	container := NewContainer("deploy", "myapp:v1",
//	    WithEnvMap(map[string]string{
//	        "LOG_LEVEL": "debug",
//	        "ENV": "production",
//	    }))
func WithEnvMap(envMap map[string]string) ContainerOption {
	return func(c *Container) {
		if c.env == nil {
			c.env = make(map[string]string)
		}
		for k, v := range envMap {
			c.env[k] = v
		}
	}
}

// WithPorts adds port mappings.
//
// Example:
//
//	container := NewContainer("server", "myapp:v1",
//	    WithPorts("8080:8080", "9090:9090"))
func WithPorts(ports ...string) ContainerOption {
	return func(c *Container) {
		c.ports = append(c.ports, ports...)
	}
}

// WithVolume adds a volume mount.
//
// Example:
//
//	container := NewContainer("build", "golang:1.25",
//	    WithVolume("/host/src", "/container/src"))
func WithVolume(hostPath, containerPath string) ContainerOption {
	return func(c *Container) {
		if c.volumes == nil {
			c.volumes = make(map[string]string)
		}
		c.volumes[hostPath] = containerPath
	}
}

// WithVolumes adds multiple volume mounts.
//
// Example:
//
//	container := NewContainer("build", "golang:1.25",
//	    WithVolumes(map[string]string{
//	        "/host/src": "/container/src",
//	        "/host/cache": "/container/cache",
//	    }))
func WithVolumes(volumes map[string]string) ContainerOption {
	return func(c *Container) {
		if c.volumes == nil {
			c.volumes = make(map[string]string)
		}
		for k, v := range volumes {
			c.volumes[k] = v
		}
	}
}

// WithWorkDir sets the working directory.
//
// Example:
//
//	container := NewContainer("build", "golang:1.25",
//	    WithWorkDir("/workspace"))
func WithWorkDir(dir string) ContainerOption {
	return func(c *Container) {
		c.workDir = dir
	}
}

// WithUser sets the user to run the container as.
//
// Example:
//
//	container := NewContainer("process", "myapp:v1",
//	    WithUser("nobody"))
func WithUser(user string) ContainerOption {
	return func(c *Container) {
		c.user = user
	}
}

// WithAutoRemove sets auto-remove behavior.
//
// Example:
//
//	container := NewContainer("build", "golang:1.25",
//	    WithAutoRemove(false))
func WithAutoRemove(autoRemove bool) ContainerOption {
	return func(c *Container) {
		c.autoRemove = autoRemove
	}
}

// WithLabel adds a label to the container.
//
// Example:
//
//	container := NewContainer("deploy", "myapp:v1",
//	    WithLabel("app", "myapp"),
//	    WithLabel("env", "production"))
func WithLabel(key, value string) ContainerOption {
	return func(c *Container) {
		if c.labels == nil {
			c.labels = make(map[string]string)
		}
		c.labels[key] = value
	}
}

// WithLabels adds multiple labels.
//
// Example:
//
//	container := NewContainer("deploy", "myapp:v1",
//	    WithLabels(map[string]string{
//	        "app": "myapp",
//	        "env": "production",
//	    }))
func WithLabels(labels map[string]string) ContainerOption {
	return func(c *Container) {
		if c.labels == nil {
			c.labels = make(map[string]string)
		}
		for k, v := range labels {
			c.labels[k] = v
		}
	}
}

// WithWaitStrategy sets the wait strategy for container readiness.
//
// Example:
//
//	container := NewContainer("postgres", "postgres:16-alpine",
//	    WithWaitStrategy(docker.WaitStrategyConfig{
//	        Type: "log",
//	        LogMessage: "ready to accept connections",
//	    }))
func WithWaitStrategy(strategy docker.WaitStrategyConfig) ContainerOption {
	return func(c *Container) {
		c.waitStrategy = strategy
	}
}

// WithWaitForLog sets a log-based wait strategy.
//
// Example:
//
//	container := NewContainer("postgres", "postgres:16-alpine",
//	    WithWaitForLog("ready to accept connections"))
func WithWaitForLog(logMessage string) ContainerOption {
	return func(c *Container) {
		c.waitStrategy = docker.WaitStrategyConfig{
			Type:       "log",
			LogMessage: logMessage,
		}
	}
}

// WithWaitForPort sets a port-based wait strategy.
//
// Example:
//
//	container := NewContainer("postgres", "postgres:16-alpine",
//	    WithWaitForPort("5432"))
func WithWaitForPort(port string) ContainerOption {
	return func(c *Container) {
		c.waitStrategy = docker.WaitStrategyConfig{
			Type: "port",
			Port: port,
		}
	}
}

// WithWaitForHTTP sets an HTTP-based wait strategy.
//
// Example:
//
//	container := NewContainer("api", "myapp:v1",
//	    WithWaitForHTTP("8080", "/health", 200))
func WithWaitForHTTP(port, path string, expectedStatus int) ContainerOption {
	return func(c *Container) {
		c.waitStrategy = docker.WaitStrategyConfig{
			Type:       "http",
			Port:       port,
			HTTPPath:   path,
			HTTPStatus: expectedStatus,
		}
	}
}

// WithWaitForHealthy sets a health check-based wait strategy.
//
// Example:
//
//	container := NewContainer("postgres", "postgres:16-alpine",
//	    WithWaitForHealthy())
func WithWaitForHealthy() ContainerOption {
	return func(c *Container) {
		c.waitStrategy = docker.WaitStrategyConfig{
			Type: "healthy",
		}
	}
}

// Validate validates the container configuration.
func (c *Container) Validate() error {
	if c.name == "" {
		return fmt.Errorf("container name is required")
	}
	if c.image == "" {
		return fmt.Errorf("container image is required")
	}
	return nil
}
