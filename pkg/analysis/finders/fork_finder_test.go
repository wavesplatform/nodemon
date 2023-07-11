package finders

import (
	"fmt"
	"log"
	"testing"
	"time"

	"nodemon/pkg/entities"
	"nodemon/pkg/storing/events"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wavesplatform/gowaves/pkg/crypto"
	"github.com/wavesplatform/gowaves/pkg/proto"
	zapLogger "go.uber.org/zap"
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
		r[i] = entities.NewStateHashEvent(node, ts, "V", h, &sh, 1)
	}
	return r
}

func TestFindLastCommonBlock(t *testing.T) {
	zap, logErr := zapLogger.NewDevelopment()
	if logErr != nil {
		log.Fatalf("can't initialize zap logger: %v", logErr)
	}
	defer func(zap *zapLogger.Logger) {
		if syncErr := zap.Sync(); syncErr != nil {
			log.Println(syncErr)
		}
	}(zap)

	forkA := generateStateHashes(0, 5)
	forkB := generateStateHashes(50, 5)
	for i, test := range []struct {
		eventsA         []entities.Event
		eventsB         []entities.Event
		error           bool
		expectedHeight  int
		expectedBlockID proto.BlockID
	}{
		{
			eventsA: mkEvents("A", 1, forkA...),
			eventsB: mkEvents("B", 1, forkB...),
			error:   true,
		},
		{
			eventsA: mkEvents("A", 11, forkA...),
			eventsB: mkEvents("B", 11, forkB...),
			error:   true,
		},
		{
			eventsA:         mkEvents("A", 1, forkA...),
			eventsB:         mkEvents("B", 1, forkA...),
			error:           false,
			expectedHeight:  5,
			expectedBlockID: forkA[4].id,
		},
		{
			eventsA:         mkEvents("A", 11, forkA...),
			eventsB:         mkEvents("B", 11, forkA...),
			error:           false,
			expectedHeight:  15,
			expectedBlockID: forkA[4].id,
		},
		{
			eventsA:         mkEvents("A", 1, forkA...),
			eventsB:         mkEvents("B", 1, forkA[0], forkA[1], forkA[2], forkB[3], forkB[4]),
			error:           false,
			expectedHeight:  3,
			expectedBlockID: forkA[2].id,
		},
		{
			eventsA:         mkEvents("A", 11, forkA...),
			eventsB:         mkEvents("B", 11, forkA[0], forkA[1], forkA[2], forkB[3], forkB[4]),
			error:           false,
			expectedHeight:  13,
			expectedBlockID: forkA[2].id,
		},
		{
			eventsA:         mkEvents("A", 2, forkA[1:]...),
			eventsB:         mkEvents("B", 1, forkA[0], forkA[1], forkA[2], forkB[3]),
			error:           false,
			expectedHeight:  3,
			expectedBlockID: forkA[2].id,
		},
		{
			eventsA:         mkEvents("A", 12, forkA[1:]...),
			eventsB:         mkEvents("B", 11, forkA[0], forkA[1], forkA[2], forkB[3]),
			error:           false,
			expectedHeight:  13,
			expectedBlockID: forkA[2].id,
		},
		{
			eventsA:         mkEvents("A", 1, forkA...),
			eventsB:         mkEvents("B", 2, forkA[1], forkA[2], forkB[3], forkB[4]),
			error:           false,
			expectedHeight:  3,
			expectedBlockID: forkA[2].id,
		},
		{
			eventsA:         mkEvents("A", 11, forkA...),
			eventsB:         mkEvents("B", 12, forkA[1], forkA[2], forkB[3], forkB[4]),
			error:           false,
			expectedHeight:  13,
			expectedBlockID: forkA[2].id,
		},
		{
			eventsA:         mkEvents("A", 2, forkA[1:]...),
			eventsB:         mkEvents("B", 1, forkA[0], forkA[1], forkA[2], forkB[3], forkB[4]),
			error:           false,
			expectedHeight:  3,
			expectedBlockID: forkA[2].id,
		},
		{
			eventsA:         mkEvents("A", 12, forkA[1:]...),
			eventsB:         mkEvents("B", 11, forkA[0], forkA[1], forkA[2], forkB[3], forkB[4]),
			error:           false,
			expectedHeight:  13,
			expectedBlockID: forkA[2].id,
		},
	} {
		t.Run(fmt.Sprintf("#%d", i+1), func(t *testing.T) {
			storage, err := events.NewStorage(10*time.Minute, zap)
			require.NoError(t, err)
			loadEvents(t, storage, test.eventsA, test.eventsB)
			ff := NewForkFinder(storage)
			h, id, err := ff.FindLastCommonBlock("A", "B")
			if test.error {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, test.expectedHeight, h)
				require.Equal(t, test.expectedBlockID, id)
			}
		})
	}
}

func TestFindLastCommonStateHash(t *testing.T) {
	zap, logErr := zapLogger.NewDevelopment()
	if logErr != nil {
		log.Fatalf("can't initialize zap logger: %v", logErr)
	}
	defer func(zap *zapLogger.Logger) {
		if syncErr := zap.Sync(); syncErr != nil {
			log.Println(syncErr)
		}
	}(zap)

	forkA := generateStateHashes(0, 5)
	forkB := generateStateHashes(50, 5)
	for i, test := range []struct {
		eventsA            []entities.Event
		eventsB            []entities.Event
		error              bool
		expectedHeight     int
		expectedStateHash  proto.StateHash
		linearSearchParams *linearSearchParams
	}{
		{
			eventsA: mkEvents("A", 1, forkA...),
			eventsB: mkEvents("B", 1, forkB...),
			error:   true,
		},
		{
			eventsA: mkEvents("A", 11, forkA...),
			eventsB: mkEvents("B", 11, forkB...),
			error:   true,
		},
		{
			eventsA:           mkEvents("A", 1, forkA...),
			eventsB:           mkEvents("B", 1, forkA...),
			error:             false,
			expectedHeight:    5,
			expectedStateHash: forkA[4].sh,
		},
		{
			eventsA:           mkEvents("A", 11, forkA...),
			eventsB:           mkEvents("B", 11, forkA...),
			error:             false,
			expectedHeight:    15,
			expectedStateHash: forkA[4].sh,
		},
		{
			eventsA:           mkEvents("A", 1, forkA...),
			eventsB:           mkEvents("B", 1, forkA[0], forkA[1], forkA[2], forkB[3], forkB[4]),
			error:             false,
			expectedHeight:    3,
			expectedStateHash: forkA[2].sh,
		},
		{
			eventsA:           mkEvents("A", 11, forkA...),
			eventsB:           mkEvents("B", 11, forkA[0], forkA[1], forkA[2], forkB[3], forkB[4]),
			error:             false,
			expectedHeight:    13,
			expectedStateHash: forkA[2].sh,
		},
		{
			eventsA:           mkEvents("A", 2, forkA[1:]...),
			eventsB:           mkEvents("B", 1, forkA[0], forkA[1], forkA[2], forkB[3]),
			error:             false,
			expectedHeight:    3,
			expectedStateHash: forkA[2].sh,
		},
		{
			eventsA:           mkEvents("A", 12, forkA[1:]...),
			eventsB:           mkEvents("B", 11, forkA[0], forkA[1], forkA[2], forkB[3]),
			error:             false,
			expectedHeight:    13,
			expectedStateHash: forkA[2].sh,
		},
		{
			eventsA:           mkEvents("A", 1, forkA...),
			eventsB:           mkEvents("B", 2, forkA[1], forkA[2], forkB[3], forkB[4]),
			error:             false,
			expectedHeight:    3,
			expectedStateHash: forkA[2].sh,
		},
		{
			eventsA:           mkEvents("A", 11, forkA...),
			eventsB:           mkEvents("B", 12, forkA[1], forkA[2], forkB[3], forkB[4]),
			error:             false,
			expectedHeight:    13,
			expectedStateHash: forkA[2].sh,
		},
		{
			eventsA:           mkEvents("A", 2, forkA[1:]...),
			eventsB:           mkEvents("B", 1, forkA[0], forkA[1], forkA[2], forkB[3], forkB[4]),
			error:             false,
			expectedHeight:    3,
			expectedStateHash: forkA[2].sh,
		},
		{
			eventsA:           mkEvents("A", 12, forkA[1:]...),
			eventsB:           mkEvents("B", 11, forkA[0], forkA[1], forkA[2], forkB[3], forkB[4]),
			error:             false,
			expectedHeight:    13,
			expectedStateHash: forkA[2].sh,
		},
		{
			eventsA:            mkEvents("A", 12, forkA[1:]...),
			eventsB:            mkEvents("B", 11, forkA[0], forkA[1], forkA[2], forkB[3], forkB[4]),
			error:              false,
			expectedHeight:     13,
			expectedStateHash:  forkA[2].sh,
			linearSearchParams: &linearSearchParams{searchDepth: 4},
		},
	} {
		t.Run(fmt.Sprintf("#%d", i+1), func(t *testing.T) {
			storage, err := events.NewStorage(10*time.Minute, zap)
			require.NoError(t, err)
			loadEvents(t, storage, test.eventsA, test.eventsB)
			ff := NewForkFinder(storage)
			if lsp := test.linearSearchParams; lsp != nil {
				ff = ff.WithLinearSearchParams(lsp.searchDepth)
			}
			h, sh, err := ff.FindLastCommonStateHash("A", "B")
			if test.error {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, test.expectedHeight, h)
				require.Equal(t, test.expectedStateHash, sh)
			}
		})
	}
}

func TestErrNoStateHashError(t *testing.T) {
	require.Equal(t, events.ErrNoFullStatement, ErrNoFullStatement)
}
