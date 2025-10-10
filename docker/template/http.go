package template

import (
	"fmt"
	"strings"

	"github.com/jasoet/go-wf/docker/payload"
)

// HTTP is a WorkflowSource that creates an HTTP request using a container.
// It uses curl to make HTTP requests for health checks, webhooks, or API calls.
//
// Example:
//
//	http := NewHTTP("health-check",
//	    WithHTTPURL("https://myapp/health"),
//	    WithHTTPMethod("GET"),
//	    WithHTTPExpectedStatus(200))
type HTTP struct {
	name           string
	url            string
	method         string
	headers        map[string]string
	body           string
	expectedStatus int
	timeoutSec     int
	autoRemove     bool
	curlImage      string
	followRedirect bool
	insecure       bool
	env            map[string]string
}

// NewHTTP creates a new HTTP workflow source.
//
// Parameters:
//   - name: Step name
//   - opts: Optional configuration functions
//
// Example:
//
//	http := NewHTTP("api-call",
//	    WithHTTPURL("https://api.example.com/v1/resource"),
//	    WithHTTPMethod("POST"),
//	    WithHTTPBody(`{"key": "value"}`))
func NewHTTP(name string, opts ...HTTPOption) *HTTP {
	h := &HTTP{
		name:           name,
		method:         "GET",
		headers:        make(map[string]string),
		expectedStatus: 200,
		timeoutSec:     30,
		autoRemove:     true,
		curlImage:      "curlimages/curl:latest",
		followRedirect: true,
		insecure:       false,
		env:            make(map[string]string),
	}

	// Apply options
	for _, opt := range opts {
		opt(h)
	}

	return h
}

// ToInput implements WorkflowSource interface.
func (h *HTTP) ToInput() payload.ContainerExecutionInput {
	// Build curl command
	curlArgs := []string{
		"-X", h.method,
		"-w", `\n%{http_code}`, // Write HTTP status code
		"--max-time", fmt.Sprintf("%d", h.timeoutSec),
	}

	// Add follow redirect flag
	if h.followRedirect {
		curlArgs = append(curlArgs, "-L")
	}

	// Add insecure flag
	if h.insecure {
		curlArgs = append(curlArgs, "-k")
	}

	// Add headers
	for key, value := range h.headers {
		curlArgs = append(curlArgs, "-H", fmt.Sprintf("%s: %s", key, value))
	}

	// Add body if present
	if h.body != "" {
		curlArgs = append(curlArgs, "-d", h.body)
	}

	// Add URL
	curlArgs = append(curlArgs, h.url)

	// Create validation script
	script := h.buildValidationScript(curlArgs)

	input := payload.ContainerExecutionInput{
		Image:      h.curlImage,
		Command:    []string{"sh", "-c"},
		Entrypoint: nil,
		Env:        h.env,
		AutoRemove: h.autoRemove,
		Name:       h.name,
	}

	// Add the script as arguments
	input.Command = append(input.Command, script)

	return input
}

// buildValidationScript creates a shell script that makes the HTTP request
// and validates the response status code.
func (h *HTTP) buildValidationScript(curlArgs []string) string {
	// Escape single quotes in curl arguments
	escapedArgs := make([]string, len(curlArgs))
	for i, arg := range curlArgs {
		escapedArgs[i] = strings.ReplaceAll(arg, "'", "'\\''")
	}

	curlCmd := "curl " + strings.Join(escapedArgs, " ")

	script := fmt.Sprintf(`
OUTPUT=$(mktemp)
%s > $OUTPUT 2>&1
EXIT_CODE=$?

if [ $EXIT_CODE -ne 0 ]; then
    echo "curl command failed with exit code $EXIT_CODE"
    cat $OUTPUT
    exit $EXIT_CODE
fi

# Extract status code (last line)
STATUS_CODE=$(tail -n 1 $OUTPUT)
# Get response body (everything except last line)
RESPONSE=$(head -n -1 $OUTPUT)

echo "Response: $RESPONSE"
echo "Status Code: $STATUS_CODE"

# Check if status code matches expected
if [ "$STATUS_CODE" = "%d" ]; then
    echo "Success: Received expected status code %d"
    exit 0
else
    echo "Error: Expected status code %d but got $STATUS_CODE"
    exit 1
fi
`, curlCmd, h.expectedStatus, h.expectedStatus, h.expectedStatus)

	return script
}

// WithHTTPURL sets the HTTP URL.
//
// Example:
//
//	http := NewHTTP("api-call", WithHTTPURL("https://api.example.com/health"))
func WithHTTPURL(url string) HTTPOption {
	return func(h *HTTP) {
		h.url = url
	}
}

// WithHTTPMethod sets the HTTP method.
//
// Example:
//
//	http := NewHTTP("api-call", WithHTTPMethod("POST"))
func WithHTTPMethod(method string) HTTPOption {
	return func(h *HTTP) {
		h.method = method
	}
}

// WithHTTPHeader adds an HTTP header.
//
// Example:
//
//	http := NewHTTP("api-call",
//	    WithHTTPHeader("Content-Type", "application/json"),
//	    WithHTTPHeader("Authorization", "Bearer token"))
func WithHTTPHeader(name, value string) HTTPOption {
	return func(h *HTTP) {
		if h.headers == nil {
			h.headers = make(map[string]string)
		}
		h.headers[name] = value
	}
}

// WithHTTPHeaders adds multiple HTTP headers.
//
// Example:
//
//	http := NewHTTP("api-call",
//	    WithHTTPHeaders(map[string]string{
//	        "Content-Type": "application/json",
//	        "Authorization": "Bearer token",
//	    }))
func WithHTTPHeaders(headers map[string]string) HTTPOption {
	return func(h *HTTP) {
		if h.headers == nil {
			h.headers = make(map[string]string)
		}
		for k, v := range headers {
			h.headers[k] = v
		}
	}
}

// WithHTTPBody sets the HTTP request body.
//
// Example:
//
//	http := NewHTTP("api-call",
//	    WithHTTPBody(`{"status": "complete"}`))
func WithHTTPBody(body string) HTTPOption {
	return func(h *HTTP) {
		h.body = body
	}
}

// WithHTTPExpectedStatus sets the expected HTTP status code.
// Default is 200.
//
// Example:
//
//	http := NewHTTP("api-call", WithHTTPExpectedStatus(201))
func WithHTTPExpectedStatus(status int) HTTPOption {
	return func(h *HTTP) {
		h.expectedStatus = status
	}
}

// WithHTTPTimeout sets the request timeout in seconds.
//
// Example:
//
//	http := NewHTTP("api-call", WithHTTPTimeout(60)) // 60 seconds
func WithHTTPTimeout(seconds int) HTTPOption {
	return func(h *HTTP) {
		h.timeoutSec = seconds
	}
}

// WithHTTPAutoRemove sets auto-remove behavior.
//
// Example:
//
//	http := NewHTTP("api-call", WithHTTPAutoRemove(false))
func WithHTTPAutoRemove(autoRemove bool) HTTPOption {
	return func(h *HTTP) {
		h.autoRemove = autoRemove
	}
}

// WithHTTPCurlImage sets a custom curl image.
//
// Example:
//
//	http := NewHTTP("api-call",
//	    WithHTTPCurlImage("custom/curl:latest"))
func WithHTTPCurlImage(image string) HTTPOption {
	return func(h *HTTP) {
		h.curlImage = image
	}
}

// WithHTTPFollowRedirect enables or disables following redirects.
// Default is true.
//
// Example:
//
//	http := NewHTTP("api-call", WithHTTPFollowRedirect(false))
func WithHTTPFollowRedirect(follow bool) HTTPOption {
	return func(h *HTTP) {
		h.followRedirect = follow
	}
}

// WithHTTPInsecure enables insecure SSL/TLS connections.
//
// Example:
//
//	http := NewHTTP("api-call", WithHTTPInsecure(true))
func WithHTTPInsecure(insecure bool) HTTPOption {
	return func(h *HTTP) {
		h.insecure = insecure
	}
}

// WithHTTPEnv adds an environment variable to the HTTP container.
//
// Example:
//
//	http := NewHTTP("api-call",
//	    WithHTTPEnv("DEBUG", "true"))
func WithHTTPEnv(name, value string) HTTPOption {
	return func(h *HTTP) {
		if h.env == nil {
			h.env = make(map[string]string)
		}
		h.env[name] = value
	}
}

// HTTPOption is a functional option for configuring HTTP.
type HTTPOption func(*HTTP)

// NewHTTPHealthCheck creates an HTTP health check source.
//
// Example:
//
//	healthCheck := NewHTTPHealthCheck("health-check",
//	    "https://myapp.com/health")
func NewHTTPHealthCheck(name, url string, opts ...HTTPOption) *HTTP {
	allOpts := append([]HTTPOption{
		WithHTTPURL(url),
		WithHTTPMethod("GET"),
		WithHTTPExpectedStatus(200),
	}, opts...)
	return NewHTTP(name, allOpts...)
}

// NewHTTPWebhook creates an HTTP webhook source.
//
// Example:
//
//	webhook := NewHTTPWebhook("slack-notify",
//	    "https://hooks.slack.com/services/...",
//	    `{"text": "Deployment complete"}`)
func NewHTTPWebhook(name, url, body string, opts ...HTTPOption) *HTTP {
	allOpts := append([]HTTPOption{
		WithHTTPURL(url),
		WithHTTPMethod("POST"),
		WithHTTPBody(body),
		WithHTTPHeader("Content-Type", "application/json"),
		WithHTTPExpectedStatus(200),
	}, opts...)
	return NewHTTP(name, allOpts...)
}

// Validate validates the HTTP configuration.
func (h *HTTP) Validate() error {
	if h.name == "" {
		return fmt.Errorf("HTTP name is required")
	}
	if h.url == "" {
		return fmt.Errorf("HTTP URL is required")
	}
	if h.method == "" {
		return fmt.Errorf("HTTP method is required")
	}
	return nil
}
