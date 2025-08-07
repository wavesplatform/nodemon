package criteria_test

import (
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/neilotoole/slogt"

	"nodemon/pkg/analysis/criteria"
	"nodemon/pkg/entities"
	"nodemon/pkg/storing/events"

	"github.com/stretchr/testify/require"
)

func mkIncompleteEvents(node string, startHeight, count int) []entities.Event {
	out := make([]entities.Event, count)
	for i := range count {
		out[i] = entities.NewVersionEvent(node, mkTimestamp(uint64(startHeight+i)), node+strconv.Itoa(i+1))
	}
	return out
}

func TestIncompleteCriterion_Analyze(t *testing.T) {
	logger := slogt.New(t)

	commonStateHashes := generateFiveStateHashes(250)
	tests := []struct {
		opts           *criteria.IncompleteCriterionOptions
		historyData    []entities.Event
		data           entities.NodeStatements
		timestamp      int64
		expectedAlerts []entities.IncompleteAlert
	}{
		{
			opts: &criteria.IncompleteCriterionOptions{Streak: 2, Depth: 3, ConsiderPrevUnreachableAsIncomplete: true},
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
			opts: &criteria.IncompleteCriterionOptions{Streak: 2, Depth: 3, ConsiderPrevUnreachableAsIncomplete: true},
			historyData: mergeEvents(
				mkEvents("a", 1, commonStateHashes[:len(commonStateHashes)-2]...),
				mkUnreachableEvents("a", len(commonStateHashes)-1, 1),
				mkIncompleteEvents("a", len(commonStateHashes), 1),
				mkEvents("a", uint64(len(commonStateHashes)+1), commonStateHashes[len(commonStateHashes)-1]),

				mkEvents("b", 1, commonStateHashes...),
				mkIncompleteEvents("b", len(commonStateHashes)+1, 1),
			),
			data: entities.NodeStatements{{Node: "a", Version: "a-node"}, {Node: "b", Version: "b-node"}},
			expectedAlerts: []entities.IncompleteAlert{
				{NodeStatement: entities.NodeStatement{Node: "a", Version: "a-node"}},
			},
		},
		{
			opts: &criteria.IncompleteCriterionOptions{Streak: 2, Depth: 3, ConsiderPrevUnreachableAsIncomplete: true},
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
			opts: &criteria.IncompleteCriterionOptions{Streak: 2, Depth: 3, ConsiderPrevUnreachableAsIncomplete: false},
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
			es, storErr := events.NewStorage(time.Minute, logger)
			require.NoError(t, storErr)
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
				criterion := criteria.NewIncompleteCriterion(es, test.opts, logger)
				criteriaErr := criterion.Analyze(alerts, test.data)
				require.NoError(t, criteriaErr)
			}()
			for j := range test.expectedAlerts {
				select {
				case actualAlert := <-alerts:
					incompleteAlert, ok := actualAlert.(*entities.IncompleteAlert)
					require.True(t, ok, "unexpected alert type: %T", actualAlert)
					require.Contains(t, test.expectedAlerts, *incompleteAlert, "test case #%d", j+1)
				case <-time.After(5 * time.Second):
					require.Fail(t, "timeout exceeded")
				}
			}
		})
	}
}
