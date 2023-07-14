package entities_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"nodemon/pkg/entities"
)

func TestNodeStatementsSplitByHeight_MinMaxHeight(t *testing.T) {
	tests := []struct {
		split entities.NodeStatementsSplitByHeight
		min   int
		max   int
	}{
		{
			split: entities.NodeStatementsSplitByHeight{3: {}, 1: {}, 2: {}, 8: {}, 16: {}},
			min:   1,
			max:   16,
		},
		{
			split: entities.NodeStatementsSplitByHeight{53: {}, 7: {}, 234: {}, 42: {}, 86: {}, 44: {}},
			min:   7,
			max:   234,
		},
		{
			split: entities.NodeStatementsSplitByHeight{42: {}},
			min:   42,
			max:   42,
		},
		{
			split: entities.NodeStatementsSplitByHeight{},
			min:   0,
			max:   0,
		},
	}
	for i, test := range tests {
		tcNum := i + 1

		min, max := test.split.MinMaxHeight()
		require.Equal(t, test.min, min, "test case #%d", tcNum)
		require.Equal(t, test.max, max, "test case #%d", tcNum)
	}
}
