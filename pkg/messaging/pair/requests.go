package pair

type Request interface {
	requestMarker()
	RequestType() RequestPairType
}

type NodesListRequest struct {
	Specific bool
}

func (*NodesListRequest) requestMarker() {}

func (r *NodesListRequest) RequestType() RequestPairType {
	if r.Specific {
		return RequestSpecificNodeListType
	}
	return RequestNodeListType
}

type NodesStatusRequest struct {
	URLs []string
}

func (*NodesStatusRequest) requestMarker() {}

func (r *NodesStatusRequest) RequestType() RequestPairType { return RequestNodesStatusType }

type InsertNewNodeRequest struct {
	URL      string
	Specific bool
}

func (*InsertNewNodeRequest) requestMarker() {}

func (r *InsertNewNodeRequest) RequestType() RequestPairType {
	if r.Specific {
		return RequestInsertSpecificNewNodeType
	}
	return RequestInsertNewNodeType
}

type UpdateNodeRequest struct {
	URL   string
	Alias string
}

func (*UpdateNodeRequest) requestMarker() {}

func (r *UpdateNodeRequest) RequestType() RequestPairType { return RequestUpdateNodeType }

type DeleteNodeRequest struct {
	URL string
}

func (*DeleteNodeRequest) requestMarker() {}

func (r *DeleteNodeRequest) RequestType() RequestPairType { return RequestDeleteNodeType }

type NodeStatementRequest struct {
	URL    string
	Height int
}

func (r *NodeStatementRequest) RequestType() RequestPairType { return RequestNodeStatementType }

func (*NodeStatementRequest) requestMarker() {}
