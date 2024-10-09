package criteria_test

import (
	"encoding/binary"
	"fmt"
	"log"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/wavesplatform/gowaves/pkg/crypto"
	"github.com/wavesplatform/gowaves/pkg/proto"
	"go.uber.org/zap"

	"nodemon/pkg/analysis/criteria"
	"nodemon/pkg/entities"
)

func mkBlockID(i int) *proto.BlockID {
	var d crypto.Digest
	binary.BigEndian.PutUint64(d[:8], uint64(i))
	blockID := proto.NewBlockIDFromDigest(d)
	return &blockID
}

func TestChallengedBlockCriterion_Analyze(t *testing.T) {
	logger, err := zap.NewDevelopment()
	if err != nil {
		log.Fatalf("can't initialize zap logger: %v", err)
	}
	defer func(zap *zap.Logger) {
		if syncErr := zap.Sync(); syncErr != nil {
			log.Println(syncErr)
		}
	}(logger)

	tests := []struct {
		opts           *criteria.ChallengedBlockCriterionOptions
		data           entities.NodeStatements
		expectedAlerts []entities.ChallengedBlockAlert
	}{
		{
			opts: &criteria.ChallengedBlockCriterionOptions{},
			data: entities.NodeStatements{
				{Node: "a", BlockID: mkBlockID(1), Challenged: true},
				{Node: "b", BlockID: mkBlockID(1), Challenged: true},
				{Node: "c", BlockID: mkBlockID(2), Challenged: true},
				{Node: "d", BlockID: mkBlockID(0), Challenged: false},
				{Node: "e", BlockID: nil, Challenged: true},
			},
			expectedAlerts: []entities.ChallengedBlockAlert{
				{Nodes: entities.Nodes{"a", "b"}, BlockID: *mkBlockID(1)},
				{Nodes: entities.Nodes{"c"}, BlockID: *mkBlockID(2)},
			},
		},
		{
			opts: nil,
			data: entities.NodeStatements{
				{Node: "a", BlockID: mkBlockID(1), Challenged: true},
				{Node: "b", BlockID: mkBlockID(1), Challenged: true},
				{Node: "c", BlockID: mkBlockID(1), Challenged: true},
				{Node: "c", BlockID: mkBlockID(2), Challenged: true},
				{Node: "d", BlockID: mkBlockID(2), Challenged: false},
				{Node: "e", BlockID: nil, Challenged: true},
			},
			expectedAlerts: []entities.ChallengedBlockAlert{
				{Nodes: entities.Nodes{"a", "b", "c"}, BlockID: *mkBlockID(1)},
				{Nodes: entities.Nodes{"c"}, BlockID: *mkBlockID(2)},
			},
		},
	}
	for i := range tests {
		test := tests[i]
		t.Run(fmt.Sprintf("TestCase#%d", i+1), func(t *testing.T) {
			done := make(chan struct{})
			defer func() {
				select {
				case <-done:
					return // test passed
				case <-time.After(5 * time.Second):
					require.Fail(t, "timeout exceeded")
				}
			}()

			alerts := make(chan entities.Alert)
			go func() {
				defer close(done)
				criterion := criteria.NewChallengedBlockCriterion(test.opts, logger)
				criterion.Analyze(alerts, 0, slices.All(test.data))
			}()
			for j := range test.expectedAlerts {
				select {
				case actualAlert := <-alerts:
					challengedBlockAlert := *actualAlert.(*entities.ChallengedBlockAlert)
					require.Contains(t, test.expectedAlerts, challengedBlockAlert, "test case #%d", j+1)
				case <-time.After(5 * time.Second):
					require.Fail(t, "timeout exceeded")
				}
			}
		})
	}
}
