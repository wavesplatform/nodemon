package pair

import (
	"nodemon/pkg/entities"

	"github.com/wavesplatform/gowaves/pkg/proto"
)

type Response interface{ responseMarker() }

type NodesListResponse struct {
	Nodes []entities.Node `json:"nodes"`
}

type NodesStatusResponse struct {
	NodesStatus []NodeStatement `json:"nodes_status"`
	ErrMessage  string          `json:"err_message"`
}

type NodeStatementResponse struct {
	NodeStatement entities.NodeStatement `json:"node_statement"`
	ErrMessage    string                 `json:"err_message"`
}

func (nl *NodesListResponse) responseMarker() {}

func (nl *NodesStatusResponse) responseMarker() {}

func (nl *NodeStatementResponse) responseMarker() {}

type NodeStatement struct {
	URL       string              `json:"url"`
	StateHash *proto.StateHash    `json:"statehash"`
	Height    int                 `json:"height"`
	Status    entities.NodeStatus `json:"status"`
}
