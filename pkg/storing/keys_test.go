package storing

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStatementKey(t *testing.T) {
	tests := []struct {
		stringKey string
		expected  StatementKey
		err       bool
	}{
		{
			stringKey: "node:https://some-node-url.com|ts:100500",
			expected:  StatementKey{"https://some-node-url.com", 100500},
			err:       false,
		},
		{
			stringKey: "node:https://some-node-url.com|ts:-100500",
			expected:  StatementKey{"https://some-node-url.com", -100500},
			err:       false,
		},
		{
			stringKey: "node:https://some-node-url.comts:100500",
			expected:  StatementKey{},
			err:       true,
		},
		{
			stringKey: "node:https://some-node-url.com||ts:100500",
			expected:  StatementKey{},
			err:       true,
		},
		{
			stringKey: "nodehttps://some-node-url.com|ts:100500",
			expected:  StatementKey{},
			err:       true,
		},
		{
			stringKey: "node:https://some-node-url.com|ts100500",
			expected:  StatementKey{},
			err:       true,
		},
	}
	for i, test := range tests {
		actual, err := NewStatementKeyFromString(test.stringKey)
		require.Equal(t, test.expected, actual, "test_case#%d", i)
		if test.err {
			require.Error(t, err, "test_case#%d", i)
		} else {
			require.NoError(t, err, "test_case#%d", i)
			require.Equal(t, test.stringKey, actual.String(), "test_case#%d", i)
		}
	}
}
