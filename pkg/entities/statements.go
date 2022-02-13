package entities

import (
	"context"

	"github.com/wavesplatform/gowaves/pkg/proto"
)

type NodeStatus string

const (
	OK             NodeStatus = "OK"
	Incomplete     NodeStatus = "incomplete"
	Unreachable    NodeStatus = "unreachable"
	InvalidVersion NodeStatus = "invalid_version"
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

type NodeStatements []NodeStatement

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
