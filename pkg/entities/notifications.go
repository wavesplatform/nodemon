package entities

const (
	NodesGatheringCompleteNotificationType = "NodesGatheringComplete"
)

type Notification interface {
	ShortDescription() string
}

type NodesGatheringNotification interface {
	Notification
	Timestamp() int64
	Nodes() []string
}

type NodesGatheringComplete struct {
	nodes []string
	ts    int64
}

func NewNodesGatheringComplete(nodes []string, ts int64) *NodesGatheringComplete {
	return &NodesGatheringComplete{nodes: nodes, ts: ts}
}

func (n *NodesGatheringComplete) ShortDescription() string {
	return NodesGatheringCompleteNotificationType
}

func (n *NodesGatheringComplete) Nodes() []string {
	return n.nodes
}

func (n *NodesGatheringComplete) Timestamp() int64 {
	return n.ts
}
