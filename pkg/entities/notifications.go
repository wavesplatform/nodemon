package entities

type NodesGatheringNotification interface {
	Timestamp() int64
	Nodes() []string
	NodesCount() int
}

type NodesGatheringComplete struct {
	nodes []string
	ts    int64
}

func NewNodesGatheringComplete(nodes []string, ts int64) *NodesGatheringComplete {
	return &NodesGatheringComplete{nodes: nodes, ts: ts}
}

func (n *NodesGatheringComplete) Nodes() []string {
	return n.nodes
}

func (n *NodesGatheringComplete) NodesCount() int {
	return len(n.nodes)
}

func (n *NodesGatheringComplete) Timestamp() int64 {
	return n.ts
}
