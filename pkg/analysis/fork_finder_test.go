package analysis

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wavesplatform/gowaves/pkg/crypto"
	"github.com/wavesplatform/gowaves/pkg/proto"
	"nodemon/pkg/entities"
	"nodemon/pkg/storing/events"
)

type shInfo struct {
	id proto.BlockID
	sh proto.StateHash
}

func sequentialBlockID(i int) proto.BlockID {
	d := crypto.Digest{}
	d[0] = byte(i)
	return proto.NewBlockIDFromDigest(d)
}

func sequentialStateHash(blockID proto.BlockID, i int) proto.StateHash {
	d := crypto.Digest{}
	d[0] = byte(i)
	return proto.StateHash{
		BlockID:      blockID,
		SumHash:      d,
		FieldsHashes: proto.FieldsHashes{},
	}
}

func generateStateHashes(o, n int) []shInfo {
	r := make([]shInfo, n)
	for i := 0; i < n; i++ {
		id := sequentialBlockID(o + i + 1)
		sh := sequentialStateHash(id, o+i+101)
		r[i] = shInfo{id: id, sh: sh}
	}
	return r
}

func loadEvents(t *testing.T, st *events.Storage, a, b []entities.Event) {
	for i := range a {
		err := st.PutEvent(a[i])
		require.NoError(t, err)
	}
	for i := range b {
		err := st.PutEvent(b[i])
		require.NoError(t, err)
	}
}

func mkEvents(node string, startHeight int, shs ...shInfo) []entities.Event {
	r := make([]entities.Event, len(shs))
	for i := range shs {
		h := startHeight + i
		ts := int64(100 + i*100)
		sh := shs[i].sh
		r[i] = entities.NewStateHashEvent(node, ts, "V", h, &sh)
	}
	return r
}

func TestFindLastCommonBlock(t *testing.T) {
	forkA := generateStateHashes(0, 5)
	forkB := generateStateHashes(50, 5)
	for i, test := range []struct {
		eventsA         []entities.Event
		eventsB         []entities.Event
		error           bool
		expectedHeight  int
		expectedBlockID proto.BlockID
	}{
		{mkEvents("A", 1, forkA...), mkEvents("B", 1, forkB...),
			true, 0, proto.BlockID{}},
		{mkEvents("A", 11, forkA...), mkEvents("B", 11, forkB...),
			true, 0, proto.BlockID{}},
		{mkEvents("A", 1, forkA...), mkEvents("B", 1, forkA...),
			false, 5, forkA[4].id},
		{mkEvents("A", 11, forkA...), mkEvents("B", 11, forkA...),
			false, 15, forkA[4].id},
		{mkEvents("A", 1, forkA...), mkEvents("B", 1, forkA[0], forkA[1], forkA[2], forkB[3], forkB[4]),
			false, 3, forkA[2].id},
		{mkEvents("A", 11, forkA...), mkEvents("B", 11, forkA[0], forkA[1], forkA[2], forkB[3], forkB[4]),
			false, 13, forkA[2].id},
		{mkEvents("A", 2, forkA[1:]...), mkEvents("B", 1, forkA[0], forkA[1], forkA[2], forkB[3]),
			false, 3, forkA[2].id},
		{mkEvents("A", 12, forkA[1:]...), mkEvents("B", 11, forkA[0], forkA[1], forkA[2], forkB[3]),
			false, 13, forkA[2].id},
		{mkEvents("A", 1, forkA...), mkEvents("B", 2, forkA[1], forkA[2], forkB[3], forkB[4]),
			false, 3, forkA[2].id},
		{mkEvents("A", 11, forkA...), mkEvents("B", 12, forkA[1], forkA[2], forkB[3], forkB[4]),
			false, 13, forkA[2].id},
		{mkEvents("A", 2, forkA[1:]...), mkEvents("B", 1, forkA[0], forkA[1], forkA[2], forkB[3], forkB[4]),
			false, 3, forkA[2].id},
		{mkEvents("A", 12, forkA[1:]...), mkEvents("B", 11, forkA[0], forkA[1], forkA[2], forkB[3], forkB[4]),
			false, 13, forkA[2].id},
	} {
		testN := fmt.Sprintf("#%d", i+1)
		storage, err := events.NewStorage(10 * time.Minute)
		ff := NewForkFinder(storage)
		require.NoError(t, err, testN)
		loadEvents(t, storage, test.eventsA, test.eventsB)
		h, id, err := ff.FindLastCommonBlock("A", "B")
		if test.error {
			assert.Error(t, err, testN)
		} else {
			require.NoError(t, err, testN)
			require.Equal(t, test.expectedHeight, h, testN)
			require.ElementsMatch(t, test.expectedBlockID.Bytes(), id, testN)
		}
	}
}

func TestFindLastCommonStateHash(t *testing.T) {
	forkA := generateStateHashes(0, 5)
	forkB := generateStateHashes(50, 5)
	for i, test := range []struct {
		eventsA           []entities.Event
		eventsB           []entities.Event
		error             bool
		expectedHeight    int
		expectedStateHash proto.StateHash
	}{
		{mkEvents("A", 1, forkA...), mkEvents("B", 1, forkB...),
			true, 0, proto.StateHash{}},
		{mkEvents("A", 11, forkA...), mkEvents("B", 11, forkB...),
			true, 0, proto.StateHash{}},
		{mkEvents("A", 1, forkA...), mkEvents("B", 1, forkA...),
			false, 5, forkA[4].sh},
		{mkEvents("A", 11, forkA...), mkEvents("B", 11, forkA...),
			false, 15, forkA[4].sh},
		{mkEvents("A", 1, forkA...), mkEvents("B", 1, forkA[0], forkA[1], forkA[2], forkB[3], forkB[4]),
			false, 3, forkA[2].sh},
		{mkEvents("A", 11, forkA...), mkEvents("B", 11, forkA[0], forkA[1], forkA[2], forkB[3], forkB[4]),
			false, 13, forkA[2].sh},
		{mkEvents("A", 2, forkA[1:]...), mkEvents("B", 1, forkA[0], forkA[1], forkA[2], forkB[3]),
			false, 3, forkA[2].sh},
		{mkEvents("A", 12, forkA[1:]...), mkEvents("B", 11, forkA[0], forkA[1], forkA[2], forkB[3]),
			false, 13, forkA[2].sh},
		{mkEvents("A", 1, forkA...), mkEvents("B", 2, forkA[1], forkA[2], forkB[3], forkB[4]),
			false, 3, forkA[2].sh},
		{mkEvents("A", 11, forkA...), mkEvents("B", 12, forkA[1], forkA[2], forkB[3], forkB[4]),
			false, 13, forkA[2].sh},
		{mkEvents("A", 2, forkA[1:]...), mkEvents("B", 1, forkA[0], forkA[1], forkA[2], forkB[3], forkB[4]),
			false, 3, forkA[2].sh},
		{mkEvents("A", 12, forkA[1:]...), mkEvents("B", 11, forkA[0], forkA[1], forkA[2], forkB[3], forkB[4]),
			false, 13, forkA[2].sh},
	} {
		testN := fmt.Sprintf("#%d", i+1)
		storage, err := events.NewStorage(10 * time.Minute)
		ff := NewForkFinder(storage)
		require.NoError(t, err, testN)
		loadEvents(t, storage, test.eventsA, test.eventsB)
		h, sh, err := ff.FindLastCommonStateHash("A", "B")
		if test.error {
			assert.Error(t, err, testN)
		} else {
			require.NoError(t, err, testN)
			require.Equal(t, test.expectedHeight, h, testN)
			require.Equal(t, test.expectedStateHash, sh, testN)
		}
	}
}
