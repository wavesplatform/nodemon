package criteria

import (
	"fmt"
	"log"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	zapLogger "go.uber.org/zap"
	"nodemon/pkg/entities"
	"nodemon/pkg/storing/events"
)

func mkIncompleteEvents(node string, startHeight, count int) []entities.Event {
	out := make([]entities.Event, count)
	for i := 0; i < count; i++ {
		out[i] = entities.NewVersionEvent(node, mkTimestamp(startHeight+i), node+strconv.Itoa(i+1))
	}
	return out
}

func TestIncompleteCriterion_Analyze(t *testing.T) {
	zap, err := zapLogger.NewDevelopment()
	if err != nil {
		log.Fatalf("can't initialize zap logger: %v", err)
	}
	defer func(zap *zapLogger.Logger) {
		err := zap.Sync()
		if err != nil {
			log.Println(err)
		}
	}(zap)
	var (
		commonStateHashes = generateStateHashes(250, 5)
	)
	tests := []struct {
		opts           *IncompleteCriterionOptions
		historyData    []entities.Event
		data           entities.NodeStatements
		timestamp      int64
		expectedAlerts []entities.IncompleteAlert
	}{
		{
			opts: &IncompleteCriterionOptions{Streak: 2, Depth: 3, ConsiderPrevUnreachableAsIncomplete: true},
			historyData: mergeEvents(
				mkEvents("a", 1, commonStateHashes[:len(commonStateHashes)-1]...),
				mkUnreachableEvents("a", len(commonStateHashes), 1),
				mkIncompleteEvents("a", len(commonStateHashes)+1, 1),

				mkEvents("b", 1, commonStateHashes...),
				mkIncompleteEvents("b", len(commonStateHashes)+1, 1),
			),
			data: entities.NodeStatements{{Node: "a", Version: "a-node"}, {Node: "b", Version: "b-node"}},
			expectedAlerts: []entities.IncompleteAlert{
				{NodeStatement: entities.NodeStatement{Node: "a", Version: "a-node"}},
			},
		},
		{
			opts: &IncompleteCriterionOptions{Streak: 2, Depth: 3, ConsiderPrevUnreachableAsIncomplete: true},
			historyData: mergeEvents(
				mkEvents("a", 1, commonStateHashes[:len(commonStateHashes)-2]...),
				mkUnreachableEvents("a", len(commonStateHashes)-1, 1),
				mkIncompleteEvents("a", len(commonStateHashes), 1),
				mkEvents("a", len(commonStateHashes)+1, commonStateHashes[len(commonStateHashes)-1]),

				mkEvents("b", 1, commonStateHashes...),
				mkIncompleteEvents("b", len(commonStateHashes)+1, 1),
			),
			data: entities.NodeStatements{{Node: "a", Version: "a-node"}, {Node: "b", Version: "b-node"}},
			expectedAlerts: []entities.IncompleteAlert{
				{NodeStatement: entities.NodeStatement{Node: "a", Version: "a-node"}},
			},
		},
		{
			opts: &IncompleteCriterionOptions{Streak: 2, Depth: 3, ConsiderPrevUnreachableAsIncomplete: true},
			historyData: mergeEvents(
				mkEvents("a", 1, commonStateHashes[:len(commonStateHashes)-2]...),
				mkIncompleteEvents("a", len(commonStateHashes), 2),
				mkUnreachableEvents("a", len(commonStateHashes)+1, 1),

				mkEvents("b", 1, commonStateHashes[:len(commonStateHashes)-1]...),
				mkIncompleteEvents("b", len(commonStateHashes), 2),
			),
			data: entities.NodeStatements{{Node: "a", Version: "a-node"}, {Node: "b", Version: "b-node"}},
			expectedAlerts: []entities.IncompleteAlert{
				{NodeStatement: entities.NodeStatement{Node: "a", Version: "a-node"}},
				{NodeStatement: entities.NodeStatement{Node: "b", Version: "b-node"}},
			},
		},
		{
			opts: &IncompleteCriterionOptions{Streak: 2, Depth: 3, ConsiderPrevUnreachableAsIncomplete: false},
			historyData: mergeEvents(
				mkEvents("a", 1, commonStateHashes[:len(commonStateHashes)-2]...),
				mkIncompleteEvents("a", len(commonStateHashes), 1),
				mkUnreachableEvents("a", len(commonStateHashes), 1),
				mkIncompleteEvents("a", len(commonStateHashes)+1, 1),

				mkEvents("b", 1, commonStateHashes[:len(commonStateHashes)-1]...),
				mkIncompleteEvents("b", len(commonStateHashes), 2),
			),
			data: entities.NodeStatements{{Node: "a", Version: "a-node"}, {Node: "b", Version: "b-node"}},
			expectedAlerts: []entities.IncompleteAlert{
				{NodeStatement: entities.NodeStatement{Node: "b", Version: "b-node"}},
			},
		},
	}
	for i := range tests {
		test := tests[i]
		t.Run(fmt.Sprintf("TestCase#%d", i+1), func(t *testing.T) {
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
				criterion := NewIncompleteCriterion(es, test.opts, zap)
				err := criterion.Analyze(alerts, test.data)
				require.NoError(t, err)
			}()
			for j := range test.expectedAlerts {
				select {
				case actualAlert := <-alerts:
					incompleteAlert := *actualAlert.(*entities.IncompleteAlert)
					require.Contains(t, test.expectedAlerts, incompleteAlert, "test case #%d", j+1)
				case <-time.After(5 * time.Second):
					require.Fail(t, "timeout exceeded")
				}
			}
		})
	}
}
