//go:build integration

// Package testutil provides shared test helpers for integration tests.
package testutil

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.temporal.io/sdk/client"
)

// TemporalContainer holds a running Temporal test container and its client.
type TemporalContainer struct {
	Client    client.Client
	container testcontainers.Container
}

// StartTemporalContainer starts a Temporal dev server container and returns a connected client.
// Call Cleanup() when done.
func StartTemporalContainer(ctx context.Context) (*TemporalContainer, error) {
	req := testcontainers.ContainerRequest{
		Image:        "temporalio/temporal:latest",
		ExposedPorts: []string{"7233/tcp", "8233/tcp"},
		Cmd:          []string{"server", "start-dev", "--ip", "0.0.0.0"},
		WaitingFor:   wait.ForListeningPort("7233/tcp").WithStartupTimeout(60 * time.Second),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to start temporal container: %w", err)
	}

	mappedPort, err := container.MappedPort(ctx, "7233")
	if err != nil {
		_ = container.Terminate(ctx)
		return nil, fmt.Errorf("failed to get mapped port: %w", err)
	}

	host, err := container.Host(ctx)
	if err != nil {
		_ = container.Terminate(ctx)
		return nil, fmt.Errorf("failed to get host: %w", err)
	}

	hostPort := fmt.Sprintf("%s:%s", host, mappedPort.Port())
	log.Printf("Temporal container started at %s", hostPort)

	// Wait for Temporal to fully initialize.
	time.Sleep(3 * time.Second)

	c, err := client.Dial(client.Options{
		HostPort: hostPort,
	})
	if err != nil {
		_ = container.Terminate(ctx)
		return nil, fmt.Errorf("failed to create Temporal client: %w", err)
	}

	return &TemporalContainer{
		Client:    c,
		container: container,
	}, nil
}

// Cleanup stops the Temporal client and terminates the container.
func (tc *TemporalContainer) Cleanup(ctx context.Context) {
	if tc.Client != nil {
		tc.Client.Close()
	}
	if tc.container != nil {
		if err := tc.container.Terminate(ctx); err != nil {
			log.Printf("failed to terminate temporal container: %v", err)
		}
	}
}
