package analysis

import (
	"github.com/pkg/errors"
	"nodemon/pkg/storing"
)

type ForkFinder struct {
	storage *storing.EventsStorage
}

func NewForkFinder(es *storing.EventsStorage) *ForkFinder {
	return &ForkFinder{storage: es}
}

func (f *ForkFinder) FindLastCommonBlock(nodeA, nodeB string) (int, []byte, error) {
	startA, err := f.storage.EarliestStatementHeight(nodeA)
	if err != nil {
		return 0, nil, errors.Wrapf(err, "no earliest statement for node '%s'", nodeA)
	}
	startB, err := f.storage.EarliestStatementHeight(nodeB)
	if err != nil {
		return 0, nil, errors.Wrapf(err, "no earliest statement for node '%s'", nodeA)
	}
	start := max(startA, startB)
	stopA, err := f.storage.LatestStatementHeight(nodeA)
	if err != nil {
		return 0, nil, errors.Wrapf(err, "no latest statement for node '%s'", nodeA)
	}
	stopB, err := f.storage.LatestStatementHeight(nodeB)
	if err != nil {
		return 0, nil, errors.Wrapf(err, "no latest statement for node '%s'", nodeB)
	}
	stop := min(stopA, stopB)

	var r int
	for start <= stop {
		middle := (start + stop) / 2
		different, err := f.differentBlocksAt(nodeA, nodeB, middle)
		if err != nil {
			return 0, nil, err
		}
		if different {
			stop = middle - 1
			r = stop
		} else {
			start = middle + 1
			r = middle
		}
	}
	statement, err := f.storage.LastStatementsAtHeight(nodeA, r)
	if err != nil {
		return 0, nil, errors.Wrapf(err, "no block ID at height %d", r)
	}
	return r, statement.StateHash.BlockID.Bytes(), nil
}

func (f *ForkFinder) differentBlocksAt(a, b string, h int) (bool, error) {
	stA, err := f.storage.LastStatementsAtHeight(a, h)
	if err != nil {
		return false, err
	}
	stB, err := f.storage.LastStatementsAtHeight(b, h)
	if err != nil {
		return false, err
	}
	if stA.StateHash == nil {
		return false, errors.Errorf("no state hash for node '%s' at height %d", a, h)
	}
	if stB.StateHash == nil {
		return false, errors.Errorf("no state hash for node '%s' at height %d", a, h)
	}
	return stA.StateHash.BlockID != stB.StateHash.BlockID, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
