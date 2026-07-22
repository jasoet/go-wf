package workflow

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRegisterSyncDataOptions(t *testing.T) {
	opts := RegisterSyncDataOptions("nightly-sync")
	assert.Equal(t, "nightly-sync.SyncData", opts.Name)
}

func TestRegisterWorkflowOptions(t *testing.T) {
	opts := RegisterWorkflowOptions("nightly-sync")
	assert.Equal(t, "nightly-sync", opts.Name)
}
