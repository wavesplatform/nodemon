package pair

type RequestPair interface{ requestMarker() }

type NodesListRequest struct {
	Specific bool
}

type NodesStatusRequest struct {
	Urls []string
}

type InsertNewNodeRequest struct {
	Url      string
	Specific bool
}

type DeleteNodeRequest struct {
	Url string
}

type NodeStatementRequest struct {
	Url    string
	Height int
}

func (nl *NodesListRequest) requestMarker() {}

func (nl *InsertNewNodeRequest) requestMarker() {}

func (nl *DeleteNodeRequest) requestMarker() {}

func (nl *NodesStatusRequest) requestMarker() {}

func (nl *NodeStatementRequest) requestMarker() {}
