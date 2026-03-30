package entities_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"nodemon/pkg/entities"
)

func TestBaseTargetAlertMessageNoPanic(t *testing.T) {
	t.Parallel()
	alert := &entities.BaseTargetAlert{
		Timestamp: 1,
		BaseTargetValues: []entities.BaseTargetValue{
			{Node: "node1", BaseTarget: 200},
			{Node: "node2", BaseTarget: 300},
		},
		Threshold: 100,
	}
	msg := alert.Message()
	assert.Contains(t, msg, "100")
	assert.Contains(t, msg, "node1")
	assert.Contains(t, msg, "200")
	assert.Contains(t, msg, "node2")
	assert.Contains(t, msg, "300")
}

func TestBaseTargetAlertMessageEmpty(t *testing.T) {
	t.Parallel()
	alert := &entities.BaseTargetAlert{
		Threshold:        0,
		BaseTargetValues: nil,
	}
	msg := alert.Message()
	require.NotEmpty(t, msg)
}
