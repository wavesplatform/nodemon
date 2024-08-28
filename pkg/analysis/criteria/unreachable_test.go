package criteria_test

import (
	"fmt"
	"log"
	"testing"
	"time"

	"nodemon/pkg/analysis/criteria"
	"nodemon/pkg/entities"
	"nodemon/pkg/storing/events"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func mkUnreachableEvents(node string, startHeight, count int) []entities.Event {
	out := make([]entities.Event, count)
	for i := 0; i < count; i++ {
		out[i] = entities.NewUnreachableEvent(node, mkTimestamp(uint64(startHeight+i)))
	}
	return out
}

func TestUnreachableCriterion_Analyze(t *testing.T) {
	logger, logErr := zap.NewDevelopment()
	if logErr != nil {
		log.Fatalf("can't initialize zap logger: %v", logErr)
	}
	defer func(zap *zap.Logger) {
		err := zap.Sync()
		if err != nil {
			log.Println(err)
		}
	}(logger)

	commonStateHashes := generateFiveStateHashes(250)
	tests := []struct {
		opts           *criteria.UnreachableCriterionOptions
		historyData    []entities.Event
		data           entities.NodeStatements
		timestamp      int64
		expectedAlerts []entities.UnreachableAlert
	}{
		{
			opts: &criteria.UnreachableCriterionOptions{Streak: 2, Depth: 3},
			historyData: mergeEvents(
				mkEvents("a", 1, commonStateHashes[:len(commonStateHashes)-1]...),
				mkUnreachableEvents("a", len(commonStateHashes), 2),
				mkEvents("b", 1, commonStateHashes...),
				mkUnreachableEvents("b", len(commonStateHashes)+1, 1),
			),
			data: entities.NodeStatements{{Node: "a"}, {Node: "b"}},
			expectedAlerts: []entities.UnreachableAlert{
				{Node: "a", Timestamp: 0},
			},
		},
		{
			opts: &criteria.UnreachableCriterionOptions{Streak: 1, Depth: 2},
			historyData: mergeEvents(
				mkEvents("a", 1, commonStateHashes[:len(commonStateHashes)-1]...),
				mkUnreachableEvents("a", len(commonStateHashes), 2),
				mkEvents("b", 1, commonStateHashes...),
				mkUnreachableEvents("b", len(commonStateHashes)+1, 1),
			),
			data: entities.NodeStatements{{Node: "a"}, {Node: "b"}},
			expectedAlerts: []entities.UnreachableAlert{
				{Node: "a", Timestamp: 0},
				{Node: "b", Timestamp: 0},
			},
		},
	}
	for i := range tests {
		test := tests[i]
		t.Run(fmt.Sprintf("TestCase#%d", i+1), func(t *testing.T) {
			es, err := events.NewStorage(time.Minute, logger)
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
				criterion := criteria.NewUnreachableCriterion(es, test.opts, logger)
				alanyzerErr := criterion.Analyze(alerts, 0, test.data)
				require.NoError(t, alanyzerErr)
			}()
			for j := range test.expectedAlerts {
				select {
				case actualAlert := <-alerts:
					unreachableAlert := *actualAlert.(*entities.UnreachableAlert)
					require.Contains(t, test.expectedAlerts, unreachableAlert, "test case #%d", j+1)
				case <-time.After(5 * time.Second):
					require.Fail(t, "timeout exceeded")
				}
			}
		})
	}
}
