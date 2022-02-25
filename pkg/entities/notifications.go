package entities

const (
	OnPollingCompleteNotificationType = "OnPollingComplete"
)

type Notification interface {
	Type() string
}

type OnPollingComplete struct {
	nodes []string
	ts    int64
}

func NewOnPollingComplete(nodes []string, ts int64) *OnPollingComplete {
	return &OnPollingComplete{nodes: nodes, ts: ts}
}

func (n *OnPollingComplete) Type() string {
	return OnPollingCompleteNotificationType
}

func (n *OnPollingComplete) Nodes() []string {
	return n.nodes
}

func (n *OnPollingComplete) Timestamp() int64 {
	return n.ts
}
