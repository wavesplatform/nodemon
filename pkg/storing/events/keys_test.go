package events

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewStatementKeyFromString(t *testing.T) {
	tests := []struct {
		stringKey string
		expected  statementKey
		err       bool
	}{
		{
			stringKey: "node:https://some-node-url.com|ts:100500",
			expected:  statementKey{"https://some-node-url.com", 100500},
			err:       false,
		},
		{
			stringKey: "node:https://some-node-url.com|ts:-100500",
			expected:  statementKey{"https://some-node-url.com", -100500},
			err:       false,
		},
		{
			stringKey: "node:https://some-node-url.comts:100500",
			expected:  statementKey{},
			err:       true,
		},
		{
			stringKey: "node:https://some-node-url.com||ts:100500",
			expected:  statementKey{},
			err:       true,
		},
		{
			stringKey: "nodehttps://some-node-url.com|ts:100500",
			expected:  statementKey{},
			err:       true,
		},
		{
			stringKey: "node:https://some-node-url.com|ts100500",
			expected:  statementKey{},
			err:       true,
		},
	}
	for i, test := range tests {
		actual, err := newStatementKeyFromString(test.stringKey)
		require.Equal(t, test.expected, actual, "test_case#%d", i)
		if test.err {
			require.Error(t, err, "test_case#%d", i)
		} else {
			require.NoError(t, err, "test_case#%d", i)
			require.Equal(t, test.stringKey, actual.String(), "test_case#%d", i)
		}
	}
}

func TestStatementKey_String(t *testing.T) {
	tests := []struct {
		key      statementKey
		expected string
	}{
		{
			key:      statementKey{NodeUrl: "https://some-node-url.com", Timestamp: 100500},
			expected: "node:https://some-node-url.com|ts:100500",
		},
		{
			key:      statementKey{NodeUrl: "https://kek.some-node-url.com", Timestamp: 500100},
			expected: "node:https://kek.some-node-url.com|ts:500100",
		},
	}
	for _, test := range tests {
		require.Equal(t, test.expected, test.key.String())
	}
}

func BenchmarkStatementKey_String(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = statementKey{NodeUrl: "https://kek.some-node-url.com", Timestamp: 500100}.String()
	}
}

func BenchmarkNewStatementKeyFromString(b *testing.B) {
	key := statementKey{NodeUrl: "https://kek.some-node-url.com", Timestamp: 500100}.String()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = newStatementKeyFromString(key)
	}
}
