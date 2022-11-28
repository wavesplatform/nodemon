package finders

import (
	"github.com/pkg/errors"
	"github.com/wavesplatform/gowaves/pkg/proto"
	"nodemon/pkg/storing/events"
)

var (
	ErrNoCommonBlocks  = errors.New("no common blocks")
	ErrNoFullStatement = events.NoFullStatementError
)

type ForkFinder struct {
	storage            *events.Storage
	linearSearchParams *linearSearchParams
}

type linearSearchParams struct {
	currentHeight int
	searchDepth   int
}

func NewForkFinder(es *events.Storage) *ForkFinder {
	return &ForkFinder{storage: es}
}

func (f *ForkFinder) WithLinearSearchParams(currentHeight, searchDepth int) *ForkFinder {
	f.linearSearchParams = &linearSearchParams{
		currentHeight: currentHeight,
		searchDepth:   searchDepth,
	}
	return f
}

func (f *ForkFinder) FindLastCommonBlock(nodeA, nodeB string) (int, proto.BlockID, error) {
	startA, err := f.storage.EarliestHeight(nodeA)
	if err != nil {
		return 0, proto.BlockID{}, errors.Wrapf(err, "no earliest statement for node '%s'", nodeA)
	}
	startB, err := f.storage.EarliestHeight(nodeB)
	if err != nil {
		return 0, proto.BlockID{}, errors.Wrapf(err, "no earliest statement for node '%s'", nodeA)
	}
	start := max(startA, startB)
	stopA, err := f.storage.LatestHeight(nodeA)
	if err != nil {
		return 0, proto.BlockID{}, errors.Wrapf(err, "no latest statement for node '%s'", nodeA)
	}
	stopB, err := f.storage.LatestHeight(nodeB)
	if err != nil {
		return 0, proto.BlockID{}, errors.Wrapf(err, "no latest statement for node '%s'", nodeB)
	}
	stop := min(stopA, stopB)

	var r int
	for start <= stop {
		middle := (start + stop) / 2
		different, err := f.differentBlocksAt(nodeA, nodeB, middle)
		if err != nil {
			return 0, proto.BlockID{}, err
		}
		if different {
			stop = middle - 1
			r = stop
		} else {
			start = middle + 1
			r = middle
		}
	}
	initialStart := max(startA, startB)
	initialStop := min(stopA, stopB)
	if r < initialStart {
		return 0, proto.BlockID{}, errors.Wrapf(ErrNoCommonBlocks,
			"no common blocks in range [%d, %d]", initialStart, initialStop)
	}
	sh, err := f.storage.StateHashAtHeight(nodeA, r)
	if err != nil {
		return 0, proto.BlockID{}, errors.Wrapf(err, "no block ID at height %d", r)
	}
	return r, sh.BlockID, nil
}

var errNotFound = errors.New("not found")

func (f *ForkFinder) tryLinearStateHashSearch(nodeA, nodeB string, start int) (int, proto.StateHash, error) {
	if f.linearSearchParams == nil {
		return 0, proto.StateHash{}, errors.Wrap(errNotFound, "no linear search params provided")
	}
	var (
		depth = f.linearSearchParams.searchDepth
		currH = f.linearSearchParams.currentHeight
	)
	for i := 0; i < depth && currH-start-i > 0; i++ {
		h := currH - i
		different, err := f.differentStateHashesAt(nodeA, nodeB, h)
		if err != nil {
			if errors.Is(err, events.NoFullStatementError) {
				continue
			}
			return 0, proto.StateHash{}, errors.Wrapf(err,
				"failed to get statement for nodes '%s' and '%s' at height %d", nodeA, nodeB, h)
		}
		if different {
			continue
		}
		sh, err := f.storage.StateHashAtHeight(nodeA, h)
		if err != nil {
			return 0, proto.StateHash{}, errors.Wrapf(err, "no statehash at height %d", h)
		}
		return h, sh, nil
	}
	return 0, proto.StateHash{}, errors.Wrapf(errNotFound, "linear search failed for nodes '%s' and '%s'", nodeA, nodeB)
}

func (f *ForkFinder) FindLastCommonStateHash(nodeA, nodeB string) (int, proto.StateHash, error) {
	startA, err := f.storage.EarliestHeight(nodeA)
	if err != nil {
		return 0, proto.StateHash{}, errors.Wrapf(err, "no earliest statement for node '%s'", nodeA)
	}
	startB, err := f.storage.EarliestHeight(nodeB)
	if err != nil {
		return 0, proto.StateHash{}, errors.Wrapf(err, "no earliest statement for node '%s'", nodeB)
	}
	start := max(startA, startB)

	h, sh, err := f.tryLinearStateHashSearch(nodeA, nodeB, start)
	switch {
	case err != nil && !errors.Is(err, errNotFound):
		err := errors.Wrapf(err, "linear statehash search failed for nodes '%s' and '%s'", nodeA, nodeB)
		return 0, proto.StateHash{}, err
	case err == nil:
		return h, sh, nil
	}

	stopA, err := f.storage.LatestHeight(nodeA)
	if err != nil {
		return 0, proto.StateHash{}, errors.Wrapf(err, "no latest statement for node '%s'", nodeA)
	}
	stopB, err := f.storage.LatestHeight(nodeB)
	if err != nil {
		return 0, proto.StateHash{}, errors.Wrapf(err, "no latest statement for node '%s'", nodeB)
	}
	stop := min(stopA, stopB)

	var r int
	for start <= stop {
		middle := (start + stop) / 2
		different, err := f.differentStateHashesAt(nodeA, nodeB, middle)
		if err != nil {
			err := errors.Wrapf(err, "binsearch failed for nodes '%s' and '%s' at height %d", nodeA, nodeB, middle)
			return 0, proto.StateHash{}, err
		}
		if different {
			stop = middle - 1
			r = stop
		} else {
			start = middle + 1
			r = middle
		}
	}
	initialStart := max(startA, startB)
	initialStop := min(stopA, stopB)
	if r < initialStart {
		return 0, proto.StateHash{}, errors.Wrapf(ErrNoCommonBlocks,
			"no common blocks in range [%d, %d]", initialStart, initialStop,
		)
	}
	sh, err = f.storage.StateHashAtHeight(nodeA, r)
	if err != nil {
		return 0, proto.StateHash{}, errors.Wrapf(err, "no statehash ID at height %d", r)
	}
	return r, sh, nil
}

func (f *ForkFinder) differentBlocksAt(a, b string, h int) (bool, error) {
	shA, err := f.storage.StateHashAtHeight(a, h)
	if err != nil { // err can be events.NoFullStatementError
		return false, err
	}
	shB, err := f.storage.StateHashAtHeight(b, h)
	if err != nil { // err can be events.NoFullStatementError
		return false, err
	}
	return shA.BlockID != shB.BlockID, nil
}

func (f *ForkFinder) differentStateHashesAt(a, b string, h int) (bool, error) {
	shA, err := f.storage.StateHashAtHeight(a, h)
	if err != nil { // err can be events.NoFullStatementError
		return false, err
	}
	shB, err := f.storage.StateHashAtHeight(b, h)
	if err != nil { // err can be events.NoFullStatementError
		return false, err
	}
	return shA.SumHash != shB.SumHash, nil
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
