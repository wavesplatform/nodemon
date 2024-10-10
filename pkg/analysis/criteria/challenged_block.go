package criteria

import (
	"iter"

	"github.com/wavesplatform/gowaves/pkg/proto"
	"go.uber.org/zap"

	"nodemon/pkg/entities"
)

type ChallengedBlockCriterionOptions struct {
	// no options for now
}

type ChallengedBlockCriterion struct {
	opts   *ChallengedBlockCriterionOptions
	logger *zap.Logger
}

func NewChallengedBlockCriterion(opts *ChallengedBlockCriterionOptions, logger *zap.Logger) *ChallengedBlockCriterion {
	if opts == nil { // default
		opts = &ChallengedBlockCriterionOptions{}
	}
	return &ChallengedBlockCriterion{opts: opts, logger: logger}
}

type statementsSeq2 = iter.Seq2[int, entities.NodeStatement]

func (c *ChallengedBlockCriterion) Analyze(alerts chan<- entities.Alert, timestamp int64, statements statementsSeq2) {
	challengedBlocks := make(map[proto.BlockID]entities.Nodes)
	for _, statement := range statements {
		if statement.Challenged && statement.BlockID != nil {
			blockID := *statement.BlockID
			challengedBlocks[blockID] = append(challengedBlocks[blockID], statement.Node)
		}
	}
	for blockID, nodes := range challengedBlocks {
		sortedNodes := nodes.Sort()
		c.logger.Info("ChallengedBlockCriterion: challenged block detected",
			zap.Stringer("block-id", blockID),
			zap.Strings("nodes", sortedNodes),
		)
		alerts <- &entities.ChallengedBlockAlert{
			Timestamp: timestamp,
			BlockID:   blockID,
			Nodes:     sortedNodes,
		}
	}
}
