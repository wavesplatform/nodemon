package entities

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
	return "OnPollingComplete"
}

func (n *OnPollingComplete) Nodes() []string {
	return n.nodes
}

func (n *OnPollingComplete) Timestamp() int64 {
	return n.ts
}

type Alert struct {
	Description string
}

func (a *Alert) Type() string {
	return "Alert"
}

func (a *Alert) String() string {
	return a.Description
}
