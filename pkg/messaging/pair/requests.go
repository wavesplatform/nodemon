package pair

type RequestPairType byte

const (
	RequestNodeListT RequestPairType = iota + 1
	RequestInsertNewNodeT
	RequestDeleteNodeT
	RequestNodesStatus
)

type RequestPair interface{ msgRequest() }

type NodeListRequest struct {
}

type InsertNewNodeRequest struct {
	Url string
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

type NodeListResponse struct {
	Urls []string `json:"urls"`
}

type InsertNewNodeResponse struct {
}

type DeleteNodeResponse struct {
}

func (nl *NodeListResponse) MsgResponse() {}

func (nl *InsertNewNodeResponse) MsgResponse() {}

func (nl *DeleteNodeResponse) MsgResponse() {}
