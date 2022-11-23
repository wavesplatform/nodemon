package pair

import (
	"github.com/wavesplatform/gowaves/pkg/proto"
	"nodemon/pkg/entities"
)

type ResponsePair interface{ responseMarker() }

type NodesListResponse struct {
	Urls []string `json:"urls"`
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
	Url       string              `json:"url"`
	StateHash *proto.StateHash    `json:"statehash"`
	Height    int                 `json:"height"`
	Status    entities.NodeStatus `json:"status"`
}
