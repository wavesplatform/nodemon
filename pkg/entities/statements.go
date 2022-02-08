package entities

import "github.com/wavesplatform/gowaves/pkg/proto"

type NodeStatus string

const (
	OK             NodeStatus = "OK"
	Incomplete     NodeStatus = "incomplete"
	Unreachable    NodeStatus = "unreachable"
	InvalidVersion NodeStatus = "invalid_version"
)

type NodeStatement struct {
	Node      string           `json:"node"`
	Status    NodeStatus       `json:"status"`
	Version   string           `json:"version,omitempty"`
	Height    int              `json:"height,omitempty"`
	StateHash *proto.StateHash `json:"state_hash,omitempty"`
}
