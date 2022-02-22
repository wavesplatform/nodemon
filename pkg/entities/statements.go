package entities

import (
	"context"
	"sort"

	"github.com/wavesplatform/gowaves/pkg/proto"
)

type NodeStatus string

const (
	OK            NodeStatus = "OK"
	Incomplete    NodeStatus = "incomplete"
	Unreachable   NodeStatus = "unreachable"
	InvalidHeight NodeStatus = "invalid_height"
)

const (
	NodeStatementStateHashJSONFieldName = "state_hash"
)

type NodeStatement struct {
	Node      string           `json:"node"`
	Timestamp int64            `json:"timestamp"`
	Status    NodeStatus       `json:"status"`
	Version   string           `json:"version,omitempty"`
	Height    int              `json:"height,omitempty"`
	StateHash *proto.StateHash `json:"state_hash,omitempty"`
}

type (
	NodeStatements []NodeStatement
	Nodes          []string
)

func (n Nodes) Sort() Nodes {
	sort.Strings(n)
	return n
}

func (s NodeStatements) Sort(less func(left, right *NodeStatement) bool) NodeStatements {
	sort.Slice(s, func(i, j int) bool { return less(&s[i], &s[j]) })
	return s
}

func (s NodeStatements) SortByNodeAsc() NodeStatements {
	return s.Sort(func(left, right *NodeStatement) bool { return left.Node < right.Node })
}

func (s NodeStatements) Nodes() Nodes {
	if len(s) == 0 {
		return nil
	}
	out := make([]string, len(s))
	for i := range s {
		out[i] = s[i].Node
	}
	return out
}

func (s NodeStatements) SplitBySumStateHash() (NodeStatementsSplitByStateHash, NodeStatements) {
	var (
		split            = make(NodeStatementsSplitByStateHash, len(s))
		withoutStateHash NodeStatements
	)
	for _, statement := range s {
		switch statement.Status {
		case OK:
			sumHash := statement.StateHash.SumHash
			split[sumHash] = append(split[sumHash], statement)
		default:
			withoutStateHash = append(withoutStateHash, statement)
		}
	}
	return split, withoutStateHash
}

func (s NodeStatements) SplitByNodeStatus() NodeStatementsSplitByStatus {
	split := make(NodeStatementsSplitByStatus, len(s))
	for _, statement := range s {
		status := statement.Status
		split[status] = append(split[status], statement)
	}
	return split
}

func (s NodeStatements) SplitByNodeHeight() NodeStatementsSplitByHeight {
	split := make(NodeStatementsSplitByHeight, len(s))
	for _, statement := range s {
		height := statement.Height
		split[height] = append(split[height], statement)
	}
	return split
}

func (s NodeStatements) SplitByNodeVersion() NodeStatementsSplitByVersion {
	split := make(NodeStatementsSplitByVersion, len(s))
	for _, statement := range s {
		version := statement.Version
		split[version] = append(split[version], statement)
	}
	return split
}

func (s NodeStatements) Iterator() *NodeStatementsIterator {
	i := 0
	return NewNodeStatementsIteratorClosure(
		func(ctx context.Context) (NodeStatement, bool) {
			select {
			case <-ctx.Done(): // fast path
				return NodeStatement{}, false
			default:
				// continue
			}
			if i < len(s) {
				statement := s[i]
				i += 1
				return statement, true
			}
			return NodeStatement{}, false
		},
	)
}
