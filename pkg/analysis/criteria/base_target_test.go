package criteria_test

import (
	"testing"
	"time"

	"nodemon/pkg/analysis/criteria"
	"nodemon/pkg/entities"

	"github.com/stretchr/testify/require"
	"github.com/wavesplatform/gowaves/pkg/proto"
)

type baseTargetInfo struct {
	node       string
	ts         int64
	v          string
	h          uint64
	sh         *proto.StateHash
	baseTarget uint64
}

func mkBaseTargetStatements(baseTargetInfo []baseTargetInfo) entities.NodeStatements {
	var statements entities.NodeStatements
	for _, info := range baseTargetInfo {
		statement := entities.NewStateHashEvent(info.node, info.ts, info.v,
			info.h, nil, info.baseTarget, nil, nil, false).Statement()
		statements = append(statements, statement)
	}
	return statements
}

func TestBaseTargetCriterion_Analyze(t *testing.T) {
	var timestamp int64 = 1
	test := struct {
		opts           *criteria.BaseTargetCriterionOptions
		data           entities.NodeStatements
		expectedAlerts []entities.BaseTargetAlert
	}{
		opts: &criteria.BaseTargetCriterionOptions{Threshold: 20},
		data: mkBaseTargetStatements([]baseTargetInfo{
			{node: "node1", ts: timestamp, v: "v1", h: 1, sh: nil, baseTarget: 21},
			{node: "node2", ts: timestamp, v: "v1", h: 1, sh: nil, baseTarget: 16},
			{node: "node3", ts: timestamp, v: "v1", h: 1, sh: nil, baseTarget: 21},
			{node: "node4", ts: timestamp, v: "v1", h: 1, sh: nil, baseTarget: 13},
		}),
		expectedAlerts: []entities.BaseTargetAlert{
			{
				Timestamp: timestamp,
				BaseTargetValues: []entities.BaseTargetValue{
					{Node: "node1", BaseTarget: 21},
					{Node: "node2", BaseTarget: 16},
					{Node: "node3", BaseTarget: 21},
					{Node: "node4", BaseTarget: 13},
				},
				Threshold: 20,
			},
		},
	}

	t.Run("TestCase Base Target Alert", func(t *testing.T) {
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
			criterion, err := criteria.NewBaseTargetCriterion(test.opts)
			require.NoError(t, err)
			criterion.Analyze(alerts, timestamp, test.data)
		}()
		for j := range test.expectedAlerts {
			select {
			case actualAlert := <-alerts:
				baseTargetAlert, ok := actualAlert.(*entities.BaseTargetAlert)
				require.True(t, ok, "unexpected alert type: %T", actualAlert)
				require.Contains(t, test.expectedAlerts, *baseTargetAlert, "test case #%d", j+1)
			case <-time.After(5 * time.Second):
				require.Fail(t, "timeout exceeded")
			}
		}
	})
}

func TestNoBaseTargetAlertCriterion_Analyze(t *testing.T) {
	var timestamp int64 = 1
	test := struct {
		opts           *criteria.BaseTargetCriterionOptions
		data           entities.NodeStatements
		expectedAlerts []entities.BaseTargetAlert
	}{
		opts: &criteria.BaseTargetCriterionOptions{Threshold: 20},
		data: mkBaseTargetStatements([]baseTargetInfo{
			{node: "node1", ts: timestamp, v: "v1", h: 1, sh: nil, baseTarget: 19},
			{node: "node2", ts: timestamp, v: "v1", h: 1, sh: nil, baseTarget: 18},
			{node: "node3", ts: timestamp, v: "v1", h: 1, sh: nil, baseTarget: 19},
			{node: "node4", ts: timestamp, v: "v1", h: 1, sh: nil, baseTarget: 1},
		}),
		expectedAlerts: nil,
	}

	t.Run("No alert TestCase", func(t *testing.T) {
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
			defer close(alerts)
			criterion, err := criteria.NewBaseTargetCriterion(test.opts)
			require.NoError(t, err)
			criterion.Analyze(alerts, timestamp, test.data)
		}()
		select {
		case actualAlert, ok := <-alerts:
			if ok {
				baseTargetAlert, btOk := actualAlert.(*entities.BaseTargetAlert)
				require.True(t, btOk, "unexpected alert type: %T", actualAlert)
				require.Fail(t, "unexpected alert: %v", baseTargetAlert)
			}
			return
		case <-time.After(5 * time.Second):
			return
		}
	})
}
