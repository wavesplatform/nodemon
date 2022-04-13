package criteria

import (
	"encoding/binary"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/wavesplatform/gowaves/pkg/crypto"
	"github.com/wavesplatform/gowaves/pkg/proto"
	"nodemon/pkg/entities"
	"nodemon/pkg/storing/events"
)

func fillEventsStorage(t *testing.T, es *events.Storage, events []entities.Event) {
	for i, event := range events {
		err := es.PutEvent(event)
		require.NoError(t, err, "failed to put event #%d %+v", i+1, event)
	}
}

type shInfo struct {
	id proto.BlockID
	sh proto.StateHash
}

func sequentialBlockID(i int) proto.BlockID {
	d := crypto.Digest{}
	binary.BigEndian.PutUint64(d[:8], uint64(i))
	return proto.NewBlockIDFromDigest(d)
}

func sequentialStateHash(blockID proto.BlockID, i int) proto.StateHash {
	d := crypto.Digest{}
	binary.BigEndian.PutUint64(d[:8], uint64(i))
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

func mkTimestamp(height int) int64 {
	return int64(100 + height*100)
}

func mkEvents(node string, startHeight int, shs ...shInfo) []entities.Event {
	r := make([]entities.Event, len(shs))
	for i := range shs {
		h := startHeight + i
		ts := mkTimestamp(h)
		sh := shs[i].sh
		r[i] = entities.NewStateHashEvent(node, ts, "V", h, &sh)
	}
	return r
}

func mergeEvents(slices ...[]entities.Event) []entities.Event {
	var out []entities.Event
	for _, slice := range slices {
		out = append(out, slice...)
	}
	return out
}

func eventsToStatements(events []entities.Event) entities.NodeStatements {
	out := make(entities.NodeStatements, len(events))
	for i := range events {
		out[i] = events[i].Statement()
	}
	return out
}

func mergeShInfo(slices ...[]shInfo) []shInfo {
	var out []shInfo
	for _, slice := range slices {
		out = append(out, slice...)
	}
	return out
}

func TestStateHashCriterion_Analyze(t *testing.T) {
	var (
		forkA             = generateStateHashes(0, 5)
		forkB             = generateStateHashes(50, 5)
		forkC             = generateStateHashes(100, 5)
		commonStateHashes = generateStateHashes(250, 5)
	)
	tests := []struct {
		opts           *StateHashCriterionOptions
		historyData    []entities.Event
		data           entities.NodeStatements
		expectedAlerts []entities.StateHashAlert
	}{
		{
			opts: &StateHashCriterionOptions{MaxForkDepth: 1},
			historyData: mergeEvents(
				mkEvents("a", 1, mergeShInfo(commonStateHashes[:2], forkA[:1])...),
				mkEvents("b", 1, mergeShInfo(commonStateHashes[:2], forkB[:1])...),
			),
			data: eventsToStatements(mergeEvents(
				mkEvents("a", 4, forkA[1:2]...),
				mkEvents("b", 4, forkB[1:2]...),
			)),
			expectedAlerts: []entities.StateHashAlert{
				{
					CurrentGroupsHeight:       4,
					LastCommonStateHashExist:  true,
					LastCommonStateHashHeight: 2,
					LastCommonStateHash:       commonStateHashes[1].sh,
					FirstGroup: entities.StateHashGroup{
						Nodes:     entities.Nodes{"a"},
						StateHash: forkA[1].sh,
					},
					SecondGroup: entities.StateHashGroup{
						Nodes:     entities.Nodes{"b"},
						StateHash: forkB[1].sh,
					},
				},
			},
		},
		{
			opts: &StateHashCriterionOptions{MaxForkDepth: 1},
			historyData: mergeEvents(
				mkEvents("a", 1, mergeShInfo(commonStateHashes[:2], forkA[:1])...),
				mkEvents("b", 1, mergeShInfo(commonStateHashes[:2], forkB[:1])...),
				mkEvents("bb", 1, mergeShInfo(commonStateHashes[:2], forkB[:1])...),
				mkEvents("bc", 1, mergeShInfo(commonStateHashes[:2], forkB[:1])...),
			),
			data: eventsToStatements(mergeEvents(
				mkEvents("a", 4, forkA[1:2]...),
				mkEvents("b", 4, forkB[1:2]...),
				mkEvents("bb", 4, forkB[1:2]...),
				mkEvents("bc", 4, forkC[1:2]...),
			)),
			expectedAlerts: []entities.StateHashAlert{
				{
					CurrentGroupsHeight:       4,
					LastCommonStateHashExist:  true,
					LastCommonStateHashHeight: 2,
					LastCommonStateHash:       commonStateHashes[1].sh,
					FirstGroup: entities.StateHashGroup{
						Nodes:     entities.Nodes{"a"},
						StateHash: forkA[1].sh,
					},
					SecondGroup: entities.StateHashGroup{
						Nodes:     entities.Nodes{"b", "bb"},
						StateHash: forkB[1].sh,
					},
				},
				{
					CurrentGroupsHeight:       4,
					LastCommonStateHashExist:  true,
					LastCommonStateHashHeight: 2,
					LastCommonStateHash:       commonStateHashes[1].sh,
					FirstGroup: entities.StateHashGroup{
						Nodes:     entities.Nodes{"a"},
						StateHash: forkA[1].sh,
					},
					SecondGroup: entities.StateHashGroup{
						Nodes:     entities.Nodes{"bc"},
						StateHash: forkC[1].sh,
					},
				},
			},
		},
		{
			opts: &StateHashCriterionOptions{MaxForkDepth: 1},
			historyData: mergeEvents(
				mkEvents("a", 1, mergeShInfo(forkA[:1])...),
				mkEvents("b", 1, mergeShInfo(forkB[:1])...),
			),
			data: eventsToStatements(mergeEvents(
				mkEvents("a", 2, forkA[1:2]...),
				mkEvents("b", 2, forkB[1:2]...),
			)),
			expectedAlerts: []entities.StateHashAlert{
				{
					CurrentGroupsHeight:       2,
					LastCommonStateHashExist:  false,
					LastCommonStateHashHeight: 0,
					LastCommonStateHash:       proto.StateHash{},
					FirstGroup: entities.StateHashGroup{
						Nodes:     entities.Nodes{"a"},
						StateHash: forkA[1].sh,
					},
					SecondGroup: entities.StateHashGroup{
						Nodes:     entities.Nodes{"b"},
						StateHash: forkB[1].sh,
					},
				},
			},
		},
	}
	for i := range tests {
		test := tests[i]
		t.Run(fmt.Sprintf("TestCase#%d", i+1), func(t *testing.T) {
			es, err := events.NewStorage(time.Minute)
			require.NoError(t, err)
			done := make(chan struct{})
			defer func() {
				select {
				case <-done:
					require.NoError(t, es.Close())
				case <-time.After(5 * time.Second):
					require.Fail(t, "timeout exceeded")
				}
			}()
			fillEventsStorage(t, es, test.historyData)

			alerts := make(chan entities.Alert)
			go func() {
				defer close(done)
				criterion := NewStateHashCriterion(es, test.opts)
				err := criterion.Analyze(alerts, 0, test.data)
				require.NoError(t, err)
			}()
			for j := range test.expectedAlerts {
				select {
				case actualAlert := <-alerts:
					stateHashAlert := *actualAlert.(*entities.StateHashAlert)
					require.Contains(t, test.expectedAlerts, stateHashAlert, "test case #%d", j+1)
				case <-time.After(5 * time.Second):
					require.Fail(t, "timeout exceeded")
				}
			}
		})
	}
}
