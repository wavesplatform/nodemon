package api

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStatementLogWrapperStringNoPanic(t *testing.T) {
	t.Parallel()
	w := statementLogWrapper{Node: "test-node", Version: "1.0.0", Height: 42}
	result := w.String()
	require.NotEmpty(t, result)
	assert.Contains(t, result, "test-node")
	assert.Contains(t, result, "1.0.0")
	assert.Contains(t, result, "42")
}
