package entities

type Notification interface {
	Type() string
}

type OnPollingComplete struct {
	nodes []string
}

func NewOnPollingComplete(nodes []string) *OnPollingComplete {
	return &OnPollingComplete{nodes: nodes}
}

func (n *OnPollingComplete) Type() string {
	return "OnPollingComplete"
}

func (n *OnPollingComplete) Nodes() []string {
	return n.nodes
}
