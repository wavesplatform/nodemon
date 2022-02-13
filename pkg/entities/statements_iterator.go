package entities

import (
	"context"

	"github.com/wavesplatform/gowaves/pkg/crypto"
)

type NodeStatementsIterable interface {
	Iterator() *NodeStatementsIterator
}

type StatementsBasicIterator interface {
	Next() bool
	Get() NodeStatement
	Close()
}

type StatementsConsumableIterator interface {
	StatementsBasicIterator
	Consume() StatementsConsumableIterator
	Consumed() bool
}

type StatementsCollectableIterator interface {
	StatementsBasicIterator
	Collect() NodeStatements
}

type (
	NodeStatementsSplitByStatus    map[NodeStatus]NodeStatements
	NodeStatementsSplitByVersion   map[string]NodeStatements
	NodeStatementsSplitByHeight    map[int]NodeStatements
	NodeStatementsSplitByStateHash map[crypto.Digest]NodeStatements
)

func (s NodeStatementsSplitByStatus) Iterator() *NodeStatementsIterator {
	return NewNodeStatementsIteratorRoutine(func(ch chan<- NodeStatement, ctx context.Context) {
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
	})
}

func (s NodeStatementsSplitByVersion) Iterator() *NodeStatementsIterator {
	return NewNodeStatementsIteratorRoutine(func(ch chan<- NodeStatement, ctx context.Context) {
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
	})
}

func (s NodeStatementsSplitByHeight) Iterator() *NodeStatementsIterator {
	return NewNodeStatementsIteratorRoutine(func(ch chan<- NodeStatement, ctx context.Context) {
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
	})
}

func (s NodeStatementsSplitByStateHash) Iterator() *NodeStatementsIterator {
	return NewNodeStatementsIteratorRoutine(func(ch chan<- NodeStatement, ctx context.Context) {
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
	})
}

type nodeStatementsIteratorType byte

const (
	consumedIterator = iota
	closureIterator
	routineIterator
)

type NodeStatementsIterator struct {
	iter          func() (NodeStatement, bool)
	iterType      nodeStatementsIteratorType
	cancel        context.CancelFunc
	buffStatement NodeStatement
}

func NewNodeStatementsIteratorRoutine(routine func(ch chan<- NodeStatement, ctx context.Context)) *NodeStatementsIterator {
	var (
		ch          = make(chan NodeStatement)
		ctx, cancel = context.WithCancel(context.Background())
	)
	go func(ch chan<- NodeStatement, ctx context.Context) {
		defer close(ch)
		routine(ch, ctx)
	}(ch, ctx)
	return &NodeStatementsIterator{
		iter: func() (NodeStatement, bool) {
			statement, more := <-ch
			return statement, more
		},
		iterType: routineIterator,
		cancel:   cancel,
	}
}

func NewNodeStatementsIteratorClosure(iter func() (NodeStatement, bool)) *NodeStatementsIterator {
	return &NodeStatementsIterator{iter: iter, iterType: closureIterator}
}

func NewNodeStatementsIteratorWrapper(iter StatementsConsumableIterator) *NodeStatementsIterator {
	inner := iter.Consume()
	if iter, ok := inner.(*NodeStatementsIterator); ok { // fast path
		return iter
	}
	return NewNodeStatementsIteratorRoutine(func(ch chan<- NodeStatement, ctx context.Context) {
		defer inner.Close()
		select { // fast path
		case <-ctx.Done():
			return
		default:
			// continue
		}
		for inner.Next() {
			statement := inner.Get()
			select {
			case <-ctx.Done():
				return
			case ch <- statement:
				continue
			}
		}
	})
}

// Next prepares new statement
func (i *NodeStatementsIterator) Next() bool {
	if i.Consumed() {
		panic("trying to call Next() on consumed NodeStatementsIterator")
	}
	statement, more := i.iter()
	if more {
		i.buffStatement = statement
	} else {
		i.Close()
	}
	return more
}

func (i *NodeStatementsIterator) Get() NodeStatement {
	if i.Consumed() {
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

// Take consumes iterator and returns 'count' (or less) values to new NodeStatements slice.
func (i *NodeStatementsIterator) Take(count int) *NodeStatementsIterator {
	var (
		inner = i.consume()
	)
	return NewNodeStatementsIteratorRoutine(func(ch chan<- NodeStatement, ctx context.Context) {
		defer inner.Close()
		for i := 0; i < count && inner.Next(); i++ {
			statement := inner.Get()
			select {
			case <-ctx.Done():
				return
			case ch <- statement:
				continue
			}
		}
	})
}

func (i *NodeStatementsIterator) Consumed() bool {
	return i.iterType == consumedIterator
}

func (i *NodeStatementsIterator) consume() *NodeStatementsIterator {
	if i.Consumed() {
		panic("trying to call consume() on consumed NodeStatementsIterator")
	}
	iter := *i
	*i = NodeStatementsIterator{iterType: consumedIterator}
	return &iter
}

// Consume returns underlying iterator and sets new empty iterator with iterType = consumedIterator.
func (i *NodeStatementsIterator) Consume() StatementsConsumableIterator {
	return i.consume()
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

// FilterMap consumes iterator and returns new wrapped iterator with filter and map func.
func (i *NodeStatementsIterator) FilterMap(fn func(statement *NodeStatement) *NodeStatement) *NodeStatementsIterator {
	var (
		inner = i.consume()
	)
	if inner.iterType == closureIterator { // optimized iterator
		return NewNodeStatementsIteratorClosure(func() (NodeStatement, bool) {
			for inner.Next() {
				statement := inner.Get()
				newStatement := fn(&statement)
				if newStatement == nil {
					continue
				}
				return *newStatement, true
			}
			return NodeStatement{}, false
		})
	}
	return NewNodeStatementsIteratorRoutine(func(ch chan<- NodeStatement, ctx context.Context) {
		defer inner.Close()
		select { // fast path
		case <-ctx.Done():
			return
		default:
			// continue
		}
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
	})
}

// Chain consumes iterator and tails iterators and returns new wrapped iterator which is union of all iterators.
func (i *NodeStatementsIterator) Chain(tails ...StatementsConsumableIterator) *NodeStatementsIterator {
	var (
		proxy = func(iter StatementsConsumableIterator, ch chan<- NodeStatement, ctx context.Context) {
			defer iter.Close()
			select {
			case <-ctx.Done(): // fast path
				return
			default:
				// continue
			}
			for iter.Next() {
				statement := iter.Get()
				select {
				case <-ctx.Done():
					return
				case ch <- statement:
					continue
				}
			}
		}
		inner = i.consume()
	)
	return NewNodeStatementsIteratorRoutine(func(ch chan<- NodeStatement, ctx context.Context) {
		proxy(inner, ch, ctx)
		for _, iter := range tails {
			iter = iter.Consume()
			proxy(iter, ch, ctx)
		}
	})
}
