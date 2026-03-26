package datasync

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestBuildRegistration(t *testing.T) {
	source := &mockSource[int]{name: "test-source"}
	sink := &mockSink[int]{name: "test-sink"}

	job := Job[int, int]{
		Name:     "test-job",
		Source:   source,
		Mapper:   IdentityMapper[int](),
		Sink:     sink,
		Schedule: 5 * time.Minute,
		Metadata: map[string]string{"key": "value"},
	}

	reg := BuildRegistration(job, false)
	assert.Equal(t, "test-job", reg.Name)
	assert.Equal(t, 5*time.Minute, reg.Schedule)
	assert.False(t, reg.Disabled)
	assert.Equal(t, "test-source", reg.SourceName)
	assert.Equal(t, "test-sink", reg.SinkName)

	regDisabled := BuildRegistration(job, true)
	assert.True(t, regDisabled.Disabled)
}
