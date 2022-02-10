package entities

import (
	"context"

	"github.com/wavesplatform/gowaves/pkg/crypto"
	"github.com/wavesplatform/gowaves/pkg/proto"
)

type NodeStatus string

const (
	OK             NodeStatus = "OK"
	Incomplete     NodeStatus = "incomplete"
	Unreachable    NodeStatus = "unreachable"
	InvalidVersion NodeStatus = "invalid_version"
)

const (
	NodeStatementStateHashJSONFieldName = "state_hash"
)

type NodeStatement struct {
	Node      string           `json:"node"`
	Status    NodeStatus       `json:"status"`
	Version   string           `json:"version,omitempty"`
	Height    int              `json:"height,omitempty"`
	StateHash *proto.StateHash `json:"state_hash,omitempty"`
}

type NodeStatementsIter interface {
	Iterator() *NodeStatementsIterator
}

type (
	NodeStatements                 []NodeStatement
	NodeStatementsSplitByStatus    map[NodeStatus]NodeStatements
	NodeStatementsSplitByVersion   map[string]NodeStatements
	NodeStatementsSplitByHeight    map[int]NodeStatements
	NodeStatementsSplitByStateHash map[crypto.Digest]NodeStatements
)

func (s NodeStatements) Iterator() *NodeStatementsIterator {
	ch := make(chan NodeStatement)
	ctx, cancel := context.WithCancel(context.Background())

	go func(ch chan<- NodeStatement, ctx context.Context) {
		defer close(ch)
		for _, statement := range s {
			select {
			case <-ctx.Done():
				return
			case ch <- statement:
				continue
			}
		}
	}(ch, ctx)
	return &NodeStatementsIterator{
		ch:     ch,
		ctx:    ctx,
		cancel: cancel,
	}
}

func (s NodeStatementsSplitByStatus) Iterator() *NodeStatementsIterator {
	ch := make(chan NodeStatement)
	ctx, cancel := context.WithCancel(context.Background())

	go func(ch chan<- NodeStatement, ctx context.Context) {
		defer close(ch)
		for _, statements := range s {
			for _, statement := range statements {
				select {
				case <-ctx.Done():
					return
				case ch <- statement:
					continue
				}
			}
		}
	}(ch, ctx)
	return &NodeStatementsIterator{
		ch:     ch,
		ctx:    ctx,
		cancel: cancel,
	}
}

func (s NodeStatementsSplitByVersion) Iterator() *NodeStatementsIterator {
	ch := make(chan NodeStatement)
	ctx, cancel := context.WithCancel(context.Background())

	go func(ch chan<- NodeStatement, ctx context.Context) {
		defer close(ch)
		for _, statements := range s {
			for _, statement := range statements {
				select {
				case <-ctx.Done():
					return
				case ch <- statement:
					continue
				}
			}
		}
	}(ch, ctx)
	return &NodeStatementsIterator{
		ch:     ch,
		ctx:    ctx,
		cancel: cancel,
	}
}

func (s NodeStatementsSplitByHeight) Iterator() *NodeStatementsIterator {
	ch := make(chan NodeStatement)
	ctx, cancel := context.WithCancel(context.Background())

	go func(ch chan<- NodeStatement, ctx context.Context) {
		defer close(ch)
		for _, statements := range s {
			for _, statement := range statements {
				select {
				case <-ctx.Done():
					return
				case ch <- statement:
					continue
				}
			}
		}
	}(ch, ctx)
	return &NodeStatementsIterator{
		ch:     ch,
		ctx:    ctx,
		cancel: cancel,
	}
}

func (s NodeStatementsSplitByStateHash) Iterator() *NodeStatementsIterator {
	ch := make(chan NodeStatement)
	ctx, cancel := context.WithCancel(context.Background())

	go func(ch chan<- NodeStatement, ctx context.Context) {
		defer close(ch)
		for _, statements := range s {
			for _, statement := range statements {
				select {
				case <-ctx.Done():
					return
				case ch <- statement:
					continue
				}
			}
		}
	}(ch, ctx)
	return &NodeStatementsIterator{
		ch:     ch,
		ctx:    ctx,
		cancel: cancel,
	}
}

type NodeStatementsIterator struct {
	ch            <-chan NodeStatement
	ctx           context.Context
	cancel        context.CancelFunc
	buffStatement NodeStatement
	consumed      bool
}

// Next prepares new statement
func (i *NodeStatementsIterator) Next() bool {
	if i.consumed {
		panic("trying to call Next() on consumed NodeStatementsIterator")
	}
	statement, more := <-i.ch
	if more {
		i.buffStatement = statement
	} else {
		i.Close()
	}
	return more
}

func (i *NodeStatementsIterator) Get() NodeStatement {
	if i.consumed {
		panic("trying to call Get() on consumed NodeStatementsIterator")
	}
	return i.buffStatement
}

func (i *NodeStatementsIterator) Close() {
	if i.cancel != nil {
		i.cancel()
		i.cancel = nil
	}
}

// Collect consumes iterator and collects all values to new NodeStatements slice.
func (i *NodeStatementsIterator) Collect() NodeStatements {
	var (
		iter       = i.consume()
		statements NodeStatements
	)
	defer iter.Close()
	for iter.Next() {
		statement := iter.Get()
		statements = append(statements, statement)
	}
	return statements
}

// consume returns underlying iterator and sets new empty iterator with consumed = true.
func (i *NodeStatementsIterator) consume() NodeStatementsIterator {
	iter := *i
	*i = NodeStatementsIterator{consumed: true}
	return iter
}

// SplitBySumStateHash consumes iterator and splits statements by state hash.
func (i *NodeStatementsIterator) SplitBySumStateHash() (NodeStatementsSplitByStateHash, NodeStatements) {
	var (
		iter             = i.consume()
		split            = make(NodeStatementsSplitByStateHash)
		withoutStateHash NodeStatements
	)
	defer iter.Close()
	for iter.Next() {
		switch statement := iter.Get(); statement.Status {
		case OK:
			sumHash := statement.StateHash.SumHash
			split[sumHash] = append(split[sumHash], statement)
		default:
			withoutStateHash = append(withoutStateHash, statement)
		}
	}
	return split, withoutStateHash
}

// SplitByNodeStatus consumes iterator and splits statements by status.
func (i *NodeStatementsIterator) SplitByNodeStatus() NodeStatementsSplitByStatus {
	var (
		iter  = i.consume()
		split = make(NodeStatementsSplitByStatus)
	)
	defer iter.Close()
	for iter.Next() {
		statement := iter.Get()
		status := statement.Status
		split[status] = append(split[status], statement)
	}
	return split
}

// SplitByNodeHeight consumes iterator and splits statements by height.
func (i *NodeStatementsIterator) SplitByNodeHeight() NodeStatementsSplitByHeight {
	var (
		iter  = i.consume()
		split = make(NodeStatementsSplitByHeight)
	)
	defer iter.Close()
	for iter.Next() {
		statement := iter.Get()
		height := statement.Height
		split[height] = append(split[height], statement)
	}
	return split
}

// SplitByNodeVersion consumes iterator and splits statements by node version.
func (i *NodeStatementsIterator) SplitByNodeVersion() NodeStatementsSplitByVersion {
	var (
		iter  = i.consume()
		split = make(NodeStatementsSplitByVersion)
	)
	defer iter.Close()
	for iter.Next() {
		statement := iter.Get()
		version := statement.Version
		split[version] = append(split[version], statement)
	}
	return split
}

// Map consumes iterator and returns new wrapped iterator.
func (i *NodeStatementsIterator) Map(fn func(statement NodeStatement) NodeStatement) *NodeStatementsIterator {
	var (
		inner       = i.consume()
		ch          = make(chan NodeStatement)
		ctx, cancel = context.WithCancel(context.Background())
	)
	go func(inner NodeStatementsIterator, ch chan<- NodeStatement, ctx context.Context) {
		defer func() {
			close(ch)
			inner.Close()
		}()
		for inner.Next() {
			oldStatement := inner.Get()
			newStatement := fn(oldStatement)
			select {
			case <-ctx.Done():
				return
			case ch <- newStatement:
				continue
			}
		}
	}(inner, ch, ctx)
	return &NodeStatementsIterator{
		ch:     ch,
		ctx:    ctx,
		cancel: cancel,
	}
}

// Filter consumes iterator and returns new wrapped iterator with filter func.
func (i *NodeStatementsIterator) Filter(fn func(statement NodeStatement) (take bool)) *NodeStatementsIterator {
	var (
		inner       = i.consume()
		ch          = make(chan NodeStatement)
		ctx, cancel = context.WithCancel(context.Background())
	)
	go func(inner NodeStatementsIterator, ch chan<- NodeStatement, ctx context.Context) {
		defer func() {
			close(ch)
			inner.Close()
		}()
		for inner.Next() {
			statement := inner.Get()
			// we should continue here in case if next==false
			if take := fn(statement); !take {
				continue
			}
			select {
			case <-ctx.Done():
				return
			case ch <- statement:
				continue
			}
		}
	}(inner, ch, ctx)
	return &NodeStatementsIterator{
		ch:     ch,
		ctx:    ctx,
		cancel: cancel,
	}
}

// FilterMap consumes iterator and returns new wrapped iterator with filter and map func.
func (i *NodeStatementsIterator) FilterMap(fn func(statement *NodeStatement) *NodeStatement) *NodeStatementsIterator {
	var (
		inner       = i.consume()
		ch          = make(chan NodeStatement)
		ctx, cancel = context.WithCancel(context.Background())
	)
	go func(inner NodeStatementsIterator, ch chan<- NodeStatement, ctx context.Context) {
		defer func() {
			close(ch)
			inner.Close()
		}()
		for inner.Next() {
			statement := inner.Get()
			newStatement := fn(&statement)
			if newStatement == nil {
				continue
			}
			select {
			case <-ctx.Done():
				return
			case ch <- *newStatement:
				continue
			}
		}
	}(inner, ch, ctx)
	return &NodeStatementsIterator{
		ch:     ch,
		ctx:    ctx,
		cancel: cancel,
	}
}
