package entities

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

func TestNodeStatementsIterWithCombinators(t *testing.T) {
	defer goleak.VerifyNone(t)

	tests := []struct {
		iterable NodeStatementsIterable
		take     int
		expected NodeStatements
	}{
		{
			iterable: NodeStatements{
				{Node: "1", Height: 1, Version: "V3", Status: Incomplete},
				{Node: "3", Height: 2, Version: "V2", Status: OK},
				{Node: "2", Height: 3, Version: "V1", Status: OK},
			},
			expected: NodeStatements{
				{Node: "1", Height: 10, Version: "V3", Status: Incomplete},
				{Node: "2", Height: 30, Version: "V1", Status: OK},
				{Node: "1", Height: 10, Version: "V3", Status: Incomplete},
				{Node: "2", Height: 30, Version: "V1", Status: OK},
			},
			take: 4,
		},
		{
			iterable: NodeStatements{
				{Node: "1", Height: 1, Version: "V3", Status: Incomplete},
				{Node: "3", Height: 2, Version: "V2", Status: OK},
				{Node: "2", Height: 3, Version: "V1", Status: OK},
			},
			expected: NodeStatements{
				{Node: "1", Height: 10, Version: "V3", Status: Incomplete},
				{Node: "2", Height: 30, Version: "V1", Status: OK},
				{Node: "1", Height: 10, Version: "V3", Status: Incomplete},
				{Node: "2", Height: 30, Version: "V1", Status: OK},
				{Node: "1", Height: 10, Version: "V3", Status: Incomplete},
				{Node: "2", Height: 30, Version: "V1", Status: OK},
			},
			take: 100,
		},
	}
	for _, test := range tests {
		actual := test.iterable.
			Iterator().
			Chain(
				test.iterable.Iterator().FilterMap(func(statement *NodeStatement) *NodeStatement {
					return statement
				}),
				NewNodeStatementsIteratorWrapper(test.iterable.Iterator()),
			).
			FilterMap(func(statement *NodeStatement) *NodeStatement {
				return statement
			}).
			FilterMap(func(statement *NodeStatement) *NodeStatement {
				statement.Height = 10 * statement.Height
				return statement
			}).
			FilterMap(func(statement *NodeStatement) *NodeStatement {
				if statement.Height != 20 {
					return statement
				}
				return nil
			}).
			Take(test.take).
			SplitByNodeStatus().Iterator().
			SplitByNodeVersion().Iterator().
			SplitByNodeHeight().Iterator().
			Collect().Iterator().
			FilterMap(func(statement *NodeStatement) *NodeStatement {
				return statement
			}).
			Collect()
		sort.Slice(actual, func(i, j int) bool {
			return actual[i].Node > actual[j].Node
		})
		sort.Slice(test.expected, func(i, j int) bool {
			return test.expected[i].Node > test.expected[j].Node
		})
		require.Equal(t, test.expected, actual)

		iter := test.iterable.
			Iterator().
			FilterMap(func(statement *NodeStatement) *NodeStatement {
				return statement
			})
		iter.Close() // check goroutine leaks

		iter = test.iterable.
			Iterator().
			Chain(
				test.iterable.Iterator().FilterMap(func(statement *NodeStatement) *NodeStatement {
					return statement
				}),
				NewNodeStatementsIteratorWrapper(test.iterable.Iterator()),
			).
			FilterMap(func(statement *NodeStatement) *NodeStatement {
				return statement
			})
		iter.Close() // check goroutine leaks
	}
}
