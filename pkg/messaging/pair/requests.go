package pair

type Request interface{ requestMarker() }

type NodesListRequest struct {
	Specific bool
}

type NodesStatusRequest struct {
	URLs []string
}

type InsertNewNodeRequest struct {
	URL      string
	Specific bool
}

type UpdateNodeRequest struct {
	URL   string
	Alias string
}

type DeleteNodeRequest struct {
	URL string
}

type NodeStatementRequest struct {
	URL    string
	Height int
}

func (nl *NodesListRequest) requestMarker() {}

func (nl *InsertNewNodeRequest) requestMarker() {}

func (nl *UpdateNodeRequest) requestMarker() {}

func (nl *DeleteNodeRequest) requestMarker() {}

func (nl *NodesStatusRequest) requestMarker() {}

func (nl *NodeStatementRequest) requestMarker() {}
