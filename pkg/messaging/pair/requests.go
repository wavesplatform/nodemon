package pair

import (
	"github.com/wavesplatform/gowaves/pkg/proto"
	"nodemon/pkg/entities"
)

type RequestPairType byte

const (
	RequestNodeListT RequestPairType = iota + 1
	RequestSpecificNodeListT
	RequestInsertNewNodeT
	RequestInsertSpecificNewNodeT
	RequestDeleteNodeT
	RequestNodesStatus
	RequestNodesHeight
)

type RequestPair interface{ msgRequest() }

type NodeListRequest struct {
	Specific bool
}

type InsertNewNodeRequest struct {
	Url      string
	Specific bool
}

type DeleteNodeRequest struct {
	Url string
}

type NodesStatusRequest struct {
	Urls []string
}

func (nl *NodeListRequest) msgRequest() {}

func (nl *InsertNewNodeRequest) msgRequest() {}

func (nl *DeleteNodeRequest) msgRequest() {}

func (nl *NodesStatusRequest) msgRequest() {}

type ResponsePair interface{ MsgResponse() }

type NodesListResponse struct {
	Urls []string `json:"urls"`
}

type NodeStatement struct {
	Url       string              `json:"url"`
	StateHash *proto.StateHash    `json:"statehash"`
	Height    int                 `json:"height"`
	Status    entities.NodeStatus `json:"status"`
}

type NodesStatusResponse struct {
	NodesStatus []NodeStatement `json:"nodes_status"`
	Err         string
}

func (nl *NodesListResponse) MsgResponse() {}

func (nl *NodesStatusResponse) MsgResponse() {}
