//go:build integration

package docker_test

import (
	"testing"
)

// TestWorkerRegistration_Integration is tested in the full integration tests.
// Worker registration is validated by the actual integration tests that use real workers.
func TestWorkerRegistration_Integration(t *testing.T) {
	t.Skip("Worker registration is tested by TestIntegration_ExecuteContainerWorkflow and other integration tests")
}
