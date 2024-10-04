package entities

import "errors"

type NodesGatheringNotification interface {
	Timestamp() int64
	Nodes() []string
	NodesCount() int
	Error() error
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

func (n *NodesGatheringComplete) Error() error {
	return nil
}

type NodesGatheringError struct {
	err error
	ts  int64
}

func NewNodesGatheringError(err error, ts int64) *NodesGatheringError {
	return &NodesGatheringError{err: err, ts: ts}
}

func (n *NodesGatheringError) Timestamp() int64 {
	return n.ts
}

func (n *NodesGatheringError) Nodes() []string {
	return nil
}

func (n *NodesGatheringError) NodesCount() int {
	return 0
}

func (n *NodesGatheringError) Error() error {
	return n.err
}

type NodesGatheringWithError struct {
	inner NodesGatheringNotification
	err   error
}

func NewNodesGatheringWithError(inner NodesGatheringNotification, err error) *NodesGatheringWithError {
	return &NodesGatheringWithError{inner: inner, err: err}
}

func (n *NodesGatheringWithError) Timestamp() int64 {
	if n.inner != nil {
		n.inner.Timestamp()
	}
	return 0
}

func (n *NodesGatheringWithError) Nodes() []string {
	if n.inner != nil {
		n.inner.Nodes()
	}
	return nil
}

func (n *NodesGatheringWithError) NodesCount() int {
	if n.inner != nil {
		n.inner.NodesCount()
	}
	return 0
}

func (n *NodesGatheringWithError) Error() error {
	if n.inner != nil {
		return errors.Join(n.err, n.inner.Error())
	}
	return n.err
}
