package analysis

import (
	"encoding/binary"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/wavesplatform/gowaves/pkg/crypto"
	"github.com/wavesplatform/gowaves/pkg/proto"
	"nodemon/pkg/analysis/criteria"
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

func mergeShInfo(slices ...[]shInfo) []shInfo {
	var out []shInfo
	for _, slice := range slices {
		out = append(out, slice...)
	}
	return out
}

func TestAnalyzer_analyzeStateHash(t *testing.T) {
	var (
		forkA             = generateStateHashes(0, 5)
		forkB             = generateStateHashes(50, 5)
		commonStateHashes = generateStateHashes(250, 5)
	)
	tests := []struct {
		opts           *AnalyzerOptions
		historyData    []entities.Event
		data           []entities.Event
		nodes          entities.Nodes
		height         int
		expectedAlerts []entities.StateHashAlert
	}{
		{
			opts: &AnalyzerOptions{StateHashCriteriaOpts: &criteria.StateHashCriterionOptions{MaxForkDepth: 1}, BaseTargetCriterionOpts: &criteria.BaseTargetCriterionOptions{Threshold: 1}},
			historyData: mergeEvents(
				mkEvents("a", 1, mergeShInfo(commonStateHashes[:2], forkA[:2])...),
				mkEvents("b", 1, mergeShInfo(commonStateHashes[:2], forkB[:2])...),
			),
			nodes:  entities.Nodes{"a", "b"},
			height: 4,
			expectedAlerts: []entities.StateHashAlert{
				{
					Timestamp:                 mkTimestamp(4),
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
				analyzer := NewAnalyzer(es, test.opts)
				event := entities.NewOnPollingComplete(test.nodes, mkTimestamp(test.height))
				err := analyzer.analyze(alerts, event)
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
