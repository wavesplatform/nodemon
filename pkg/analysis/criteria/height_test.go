package criteria_test

import (
	"fmt"
	"log"
	"testing"
	"time"

	"nodemon/pkg/analysis/criteria"
	"nodemon/pkg/entities"

	"github.com/stretchr/testify/require"
	zapLogger "go.uber.org/zap"
)

type heightInfo struct {
	Height int
	Nodes  entities.Nodes
}

func mkHeightStatements(heightInfos []heightInfo) entities.NodeStatements {
	var statements entities.NodeStatements
	for _, info := range heightInfos {
		for _, name := range info.Nodes {
			statement := entities.NewHeightEvent(name, 0, "", info.Height).Statement()
			statements = append(statements, statement)
		}
	}
	return statements
}

func TestHeightCriterion_Analyze(t *testing.T) {
	zap, err := zapLogger.NewDevelopment()
	if err != nil {
		log.Fatalf("can't initialize zap logger: %v", err)
	}
	defer func(zap *zapLogger.Logger) {
		if syncErr := zap.Sync(); syncErr != nil {
			log.Println(syncErr)
		}
	}(zap)

	tests := []struct {
		opts           *criteria.HeightCriterionOptions
		data           entities.NodeStatements
		expectedAlerts []entities.HeightAlert
	}{
		{
			opts: &criteria.HeightCriterionOptions{MaxHeightDiff: 20},
			data: mkHeightStatements([]heightInfo{
				{Height: 1, Nodes: entities.Nodes{"n1_1", "n2_1", "n3_1"}},
				{Height: 3, Nodes: entities.Nodes{"n1_3", "n2_3", "n3_3"}},
				{Height: 15, Nodes: entities.Nodes{"n1_15", "n2_15", "n3_15"}},
				{Height: 23, Nodes: entities.Nodes{"n1_23", "n2_23", "n3_23"}},
				{Height: 24, Nodes: entities.Nodes{"n1_24", "n2_24", "n3_24"}},
			}),
			expectedAlerts: []entities.HeightAlert{
				{
					MaxHeightGroup:   entities.HeightGroup{Height: 24, Nodes: entities.Nodes{"n1_24", "n2_24", "n3_24"}},
					OtherHeightGroup: entities.HeightGroup{Height: 3, Nodes: entities.Nodes{"n1_3", "n2_3", "n3_3"}},
				},
				{
					MaxHeightGroup:   entities.HeightGroup{Height: 24, Nodes: entities.Nodes{"n1_24", "n2_24", "n3_24"}},
					OtherHeightGroup: entities.HeightGroup{Height: 1, Nodes: entities.Nodes{"n1_1", "n2_1", "n3_1"}},
				},
			},
		},
		{
			opts: &criteria.HeightCriterionOptions{MaxHeightDiff: 10},
			data: mkHeightStatements([]heightInfo{
				{Height: 25, Nodes: entities.Nodes{"n1_25", "n2_25", "n3_25"}},
				{Height: 30, Nodes: entities.Nodes{"n1_30", "n2_30", "n3_30"}},
				{Height: 40, Nodes: entities.Nodes{"n1_40", "n2_40", "n3_40"}},
			}),
			expectedAlerts: []entities.HeightAlert{
				{
					MaxHeightGroup:   entities.HeightGroup{Height: 40, Nodes: entities.Nodes{"n1_40", "n2_40", "n3_40"}},
					OtherHeightGroup: entities.HeightGroup{Height: 25, Nodes: entities.Nodes{"n1_25", "n2_25", "n3_25"}},
				},
			},
		},
	}
	for i := range tests {
		test := tests[i]
		t.Run(fmt.Sprintf("TestCase#%d", i+1), func(t *testing.T) {
			alerts := make(chan entities.Alert)
			done := make(chan struct{})
			defer func() {
				select {
				case <-done:
					// ok
				case <-time.After(5 * time.Second):
					require.Fail(t, "timeout exceeded")
				}
			}()
			go func() {
				defer close(done)
				criterion := criteria.NewHeightCriterion(test.opts, zap)
				criterion.Analyze(alerts, 0, test.data)
			}()
			for j := range test.expectedAlerts {
				select {
				case actualAlert := <-alerts:
					heightAlert := *actualAlert.(*entities.HeightAlert)
					require.Contains(t, test.expectedAlerts, heightAlert, "test case #%d", j+1)
				case <-time.After(5 * time.Second):
					require.Fail(t, "timeout exceeded")
				}
			}
		})
	}
}
