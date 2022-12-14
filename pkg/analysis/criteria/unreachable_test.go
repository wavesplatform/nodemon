package criteria

import (
	"fmt"
	"log"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	zapLogger "go.uber.org/zap"
	"nodemon/pkg/entities"
	"nodemon/pkg/storing/events"
)

func mkUnreachableEvents(node string, startHeight, count int) []entities.Event {
	out := make([]entities.Event, count)
	for i := 0; i < count; i++ {
		out[i] = entities.NewUnreachableEvent(node, mkTimestamp(startHeight+i))
	}
	return out
}

func TestUnreachableCriterion_Analyze(t *testing.T) {
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
		opts           *UnreachableCriterionOptions
		historyData    []entities.Event
		data           entities.NodeStatements
		timestamp      int64
		expectedAlerts []entities.UnreachableAlert
	}{
		{
			opts: &UnreachableCriterionOptions{Streak: 2, Depth: 3},
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
			opts: &UnreachableCriterionOptions{Streak: 1, Depth: 2},
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
				criterion := NewUnreachableCriterion(es, test.opts, zap)
				err := criterion.Analyze(alerts, 0, test.data)
				require.NoError(t, err)
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
