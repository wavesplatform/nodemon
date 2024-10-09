package analysis_test

import (
	"encoding/binary"
	"fmt"
	"log"
	"testing"
	"time"

	"nodemon/pkg/analysis"
	"nodemon/pkg/analysis/criteria"
	"nodemon/pkg/entities"
	"nodemon/pkg/storing/events"

	"github.com/stretchr/testify/require"
	"github.com/wavesplatform/gowaves/pkg/crypto"
	"github.com/wavesplatform/gowaves/pkg/proto"
	"go.uber.org/zap"
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

func mkTimestamp(height uint64) int64 {
	return int64(100 + height*100)
}

func mkEvents(node string, startHeight uint64, shs ...shInfo) []entities.Event {
	r := make([]entities.Event, len(shs))
	for i := range shs {
		h := startHeight + uint64(i)
		ts := mkTimestamp(h)
		sh := shs[i].sh
		r[i] = entities.NewStateHashEvent(node, ts, "V", h, &sh, 1, &sh.BlockID, nil, false)
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

type testCase struct {
	opts           *analysis.AnalyzerOptions
	historyData    []entities.Event
	nodes          entities.Nodes
	height         uint64
	expectedAlerts []entities.StateHashAlert
}

func TestAnalyzer_analyzeStateHash(t *testing.T) {
	logger, err := zap.NewDevelopment()
	if err != nil {
		log.Fatalf("can't initialize zap logger: %v", err)
	}
	defer func(zap *zap.Logger) {
		if syncErr := zap.Sync(); syncErr != nil {
			log.Println(syncErr)
		}
	}(logger)

	var (
		forkA             = generateStateHashes(0, 5)
		forkB             = generateStateHashes(50, 5)
		commonStateHashes = generateStateHashes(250, 5)
	)
	tests := []testCase{
		{
			opts: &analysis.AnalyzerOptions{
				StateHashCriteriaOpts:   &criteria.StateHashCriterionOptions{MaxForkDepth: 1, HeightBucketSize: 3},
				BaseTargetCriterionOpts: &criteria.BaseTargetCriterionOptions{Threshold: 2},
			},
			historyData: mergeEvents(
				mkEvents("a", 1, mergeShInfo(commonStateHashes[:1], forkA[:3])...),
				mkEvents("b", 1, mergeShInfo(commonStateHashes[:1], forkB[:3])...),
			),
			nodes:  entities.Nodes{"a", "b"},
			height: 4,
			expectedAlerts: []entities.StateHashAlert{
				{
					Timestamp:                 mkTimestamp(4),
					CurrentGroupsBucketHeight: 4,
					LastCommonStateHashExist:  true,
					LastCommonStateHashHeight: 1,
					LastCommonStateHash:       commonStateHashes[0].sh,
					FirstGroup: entities.StateHashGroup{
						Nodes:     entities.Nodes{"a"},
						StateHash: forkA[2].sh,
					},
					SecondGroup: entities.StateHashGroup{
						Nodes:     entities.Nodes{"b"},
						StateHash: forkB[2].sh,
					},
				},
			},
		},
	}
	for i := range tests {
		test := tests[i]
		t.Run(fmt.Sprintf("TestCase#%d", i+1), runTestCase(logger, test))
	}
}

func runTestCase(zap *zap.Logger, test testCase) func(t *testing.T) {
	return func(t *testing.T) {
		es, err := events.NewStorage(time.Minute, zap)
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
			analyzer := analysis.NewAnalyzer(es, test.opts, zap)
			event := entities.NewNodesGatheringComplete(test.nodes, mkTimestamp(test.height))
			notifications := make(chan entities.NodesGatheringNotification)
			analyzerOut := analyzer.Start(notifications)
			notifications <- event
			close(notifications)
			for alert := range analyzerOut {
				alerts <- alert
			}
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
	}
}
