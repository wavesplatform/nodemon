package finders

import (
	"nodemon/pkg/storing/events"

	"github.com/pkg/errors"
	"github.com/wavesplatform/gowaves/pkg/proto"
)

var (
	ErrNoCommonBlocks  = errors.New("no common blocks")
	ErrNoFullStatement = events.ErrNoFullStatement
)

type ForkFinder struct {
	storage            *events.Storage
	linearSearchParams *linearSearchParams
}

type linearSearchParams struct {
	searchDepth uint64
}

func NewForkFinder(es *events.Storage) *ForkFinder {
	return &ForkFinder{storage: es}
}

func (f *ForkFinder) WithLinearSearchParams(searchDepth uint64) *ForkFinder {
	f.linearSearchParams = &linearSearchParams{
		searchDepth: searchDepth,
	}
	return f
}

func (f *ForkFinder) FindLastCommonBlock(nodeA, nodeB string) (uint64, proto.BlockID, error) {
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

	var r uint64
	for start <= stop {
		middle := (start + stop) / 2 //nolint:mnd // count here is 2
		different, stgErr := f.differentBlocksAt(nodeA, nodeB, middle)
		if stgErr != nil {
			return 0, proto.BlockID{}, stgErr
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

func (f *ForkFinder) tryLinearStateHashSearch(
	nodeA, nodeB string, start, stop uint64,
) (uint64, proto.StateHash, error) {
	if f.linearSearchParams == nil {
		return 0, proto.StateHash{}, errors.Wrap(errNotFound, "no linear search params provided")
	}
	for i := uint64(0); i < f.linearSearchParams.searchDepth && stop-start > i; i++ {
		h := stop - i
		different, err := f.differentStateHashesAt(nodeA, nodeB, h)
		if err != nil {
			if errors.Is(err, events.ErrNoFullStatement) {
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

func (f *ForkFinder) FindLastCommonStateHash(nodeA, nodeB string) (uint64, proto.StateHash, error) {
	startA, err := f.storage.EarliestHeight(nodeA)
	if err != nil {
		return 0, proto.StateHash{}, errors.Wrapf(err, "no earliest statement for node '%s'", nodeA)
	}
	startB, err := f.storage.EarliestHeight(nodeB)
	if err != nil {
		return 0, proto.StateHash{}, errors.Wrapf(err, "no earliest statement for node '%s'", nodeB)
	}
	start := max(startA, startB)

	stopA, err := f.storage.LatestHeight(nodeA)
	if err != nil {
		return 0, proto.StateHash{}, errors.Wrapf(err, "no latest statement for node '%s'", nodeA)
	}
	stopB, err := f.storage.LatestHeight(nodeB)
	if err != nil {
		return 0, proto.StateHash{}, errors.Wrapf(err, "no latest statement for node '%s'", nodeB)
	}
	stop := min(stopA, stopB)

	h, sh, err := f.tryLinearStateHashSearch(nodeA, nodeB, start, stop)
	switch {
	case err != nil && !errors.Is(err, errNotFound):
		return 0, proto.StateHash{},
			errors.Wrapf(err, "linear statehash search failed for nodes '%s' and '%s'", nodeA, nodeB)
	case err == nil:
		return h, sh, nil
	}

	var r uint64
	for start <= stop {
		middle := (start + stop) / 2 //nolint:mnd // count here is 2
		different, stgErr := f.differentStateHashesAt(nodeA, nodeB, middle)
		if stgErr != nil {
			stgErr = errors.Wrapf(stgErr, "binsearch failed for nodes '%s' and '%s' at height %d",
				nodeA, nodeB, middle,
			)
			return 0, proto.StateHash{}, stgErr
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

func (f *ForkFinder) differentBlocksAt(a, b string, h uint64) (bool, error) {
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

func (f *ForkFinder) differentStateHashesAt(a, b string, h uint64) (bool, error) {
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
