package criteria

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/wavesplatform/gowaves/pkg/proto"
	"nodemon/pkg/entities"
)

type baseTargetInfo struct {
	node       string
	ts         int64
	v          string
	h          int
	sh         *proto.StateHash
	baseTarget int
}

func mkBaseTargetStatements(baseTargetInfo []baseTargetInfo) entities.NodeStatements {
	var statements entities.NodeStatements
	for _, info := range baseTargetInfo {
		statement := entities.NewStateHashEvent(info.node, info.ts, info.v, info.h, nil, info.baseTarget).Statement()
		statements = append(statements, statement)
	}
	return statements
}

func TestBaseTargetCriterion_Analyze(t *testing.T) {
	var timestamp int64 = 1
	tests := []struct {
		opts           *BaseTargetCriterionOptions
		data           entities.NodeStatements
		expectedAlerts []entities.BaseTargetAlert
	}{
		{
			opts: &BaseTargetCriterionOptions{Threshold: 20},
			data: mkBaseTargetStatements([]baseTargetInfo{
				{node: "node1", ts: timestamp, v: "v1", h: 1, sh: nil, baseTarget: 21},
				{node: "node2", ts: timestamp, v: "v1", h: 1, sh: nil, baseTarget: 16},
				{node: "node3", ts: timestamp, v: "v1", h: 1, sh: nil, baseTarget: 21},
				{node: "node4", ts: timestamp, v: "v1", h: 1, sh: nil, baseTarget: 13},
			}),
			expectedAlerts: []entities.BaseTargetAlert{
				{
					Timestamp:        1,
					BaseTargetValues: []entities.BaseTargetValue{{Node: "node1", BaseTarget: 21}, {Node: "node2", BaseTarget: 16}, {Node: "node3", BaseTarget: 21}, {Node: "node4", BaseTarget: 13}},
					Threshold:        20,
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
				criterion, err := NewBaseTargetCriterion(test.opts)
				require.NoError(t, err)
				criterion.Analyze(alerts, timestamp, test.data)
			}()
			for j := range test.expectedAlerts {
				select {
				case actualAlert := <-alerts:
					baseTargetAlert := *actualAlert.(*entities.BaseTargetAlert)
					require.Contains(t, test.expectedAlerts, baseTargetAlert, "test case #%d", j+1)
				case <-time.After(5 * time.Second):
					require.Fail(t, "timeout exceeded")
				}
			}
		})
	}
}
