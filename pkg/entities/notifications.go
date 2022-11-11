package entities

const (
	OnPollingCompleteNotificationType = "OnPollingComplete"
)

type Notification interface {
	ShortDescription() string
}

type OnPollingComplete struct {
	nodes []string
	Ts    int64
}

func NewOnPollingComplete(nodes []string, ts int64) *OnPollingComplete {
	return &OnPollingComplete{nodes: nodes, Ts: ts}
}

func (n *OnPollingComplete) ShortDescription() string {
	return OnPollingCompleteNotificationType
}

func (n *OnPollingComplete) Nodes() []string {
	return n.nodes
}

func (n *OnPollingComplete) Timestamp() int64 {
	return n.Ts
}

type WrappedNotification interface {
	ShortDescription() string
}
