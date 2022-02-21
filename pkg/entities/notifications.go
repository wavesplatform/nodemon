package entities

import "fmt"

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

type Alert interface {
	Notification
	Message() string
}

type SimpleAlert struct {
	message string
}

func NewSimpleAlert(message string) *SimpleAlert {
	return &SimpleAlert{message: message}
}

func (a *SimpleAlert) Type() string {
	return "SimpleAlert"
}

func (a *SimpleAlert) Message() string {
	return a.message
}

func (a *SimpleAlert) String() string {
	return fmt.Sprintf("%s: %s", a.Type(), a.Message())
}
