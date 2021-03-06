package finders

import (
	"github.com/pkg/errors"
	"github.com/wavesplatform/gowaves/pkg/proto"
	"nodemon/pkg/storing/events"
)

var (
	ErrNoCommonBlocks = errors.New("no common blocks")
)

type ForkFinder struct {
	storage *events.Storage
}

func NewForkFinder(es *events.Storage) *ForkFinder {
	return &ForkFinder{storage: es}
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
		return 0, proto.BlockID{}, errors.Errorf("no common blocks in range [%d, %d]", initialStart, initialStop)
	}
	sh, err := f.storage.LastStateHashAtHeight(nodeA, r)
	if err != nil {
		return 0, proto.BlockID{}, errors.Wrapf(err, "no block ID at height %d", r)
	}
	return r, sh.BlockID, nil
}

func (f *ForkFinder) FindLastCommonStateHash(nodeA, nodeB string) (int, proto.StateHash, error) {
	startA, err := f.storage.EarliestHeight(nodeA)
	if err != nil {
		return 0, proto.StateHash{}, errors.Wrapf(err, "no earliest statement for node '%s'", nodeA)
	}
	startB, err := f.storage.EarliestHeight(nodeB)
	if err != nil {
		return 0, proto.StateHash{}, errors.Wrapf(err, "no earliest statement for node '%s'", nodeA)
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

	var r int
	for start <= stop {
		middle := (start + stop) / 2
		different, err := f.differentStateHashesAt(nodeA, nodeB, middle)
		if err != nil {
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
	sh, err := f.storage.LastStateHashAtHeight(nodeA, r)
	if err != nil {
		return 0, proto.StateHash{}, errors.Wrapf(err, "no block ID at height %d", r)
	}
	return r, sh, nil
}

func (f *ForkFinder) differentBlocksAt(a, b string, h int) (bool, error) {
	shA, err := f.storage.LastStateHashAtHeight(a, h)
	if err != nil {
		return false, err
	}
	shB, err := f.storage.LastStateHashAtHeight(b, h)
	if err != nil {
		return false, err
	}
	return shA.BlockID != shB.BlockID, nil
}

func (f *ForkFinder) differentStateHashesAt(a, b string, h int) (bool, error) {
	shA, err := f.storage.LastStateHashAtHeight(a, h)
	if err != nil {
		return false, err
	}
	shB, err := f.storage.LastStateHashAtHeight(b, h)
	if err != nil {
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
