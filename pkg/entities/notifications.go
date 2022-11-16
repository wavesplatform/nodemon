package entities

const (
	OnPollingCompleteNotificationType = "OnPollingComplete"
)

type Notification interface {
	ShortDescription() string
}

type NodesGatheringNotification interface {
	Notification
	Timestamp() int64
	Nodes() []string
}

type OnPollingComplete struct {
	nodes []string
	ts    int64
}

func NewOnPollingComplete(nodes []string, ts int64) *OnPollingComplete {
	return &OnPollingComplete{nodes: nodes, ts: ts}
}

func (n *OnPollingComplete) ShortDescription() string {
	return OnPollingCompleteNotificationType
}

func (n *OnPollingComplete) Nodes() []string {
	return n.nodes
}

func (n *OnPollingComplete) Timestamp() int64 {
	return n.ts
}
