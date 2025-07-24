package criteria

import (
	"iter"
	"log/slog"

	"github.com/wavesplatform/gowaves/pkg/proto"

	"nodemon/pkg/entities"
	"nodemon/pkg/tools/logging/attrs"
)

type ChallengedBlockCriterionOptions struct {
	// no options for now
}

type ChallengedBlockCriterion struct {
	opts   *ChallengedBlockCriterionOptions
	logger *slog.Logger
}

func NewChallengedBlockCriterion(opts *ChallengedBlockCriterionOptions, logger *slog.Logger) *ChallengedBlockCriterion {
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
			attrs.Stringer("block_id", blockID),
			attrs.Strings("nodes", sortedNodes),
		)
		alerts <- &entities.ChallengedBlockAlert{
			Timestamp: timestamp,
			BlockID:   blockID,
			Nodes:     sortedNodes,
		}
	}
}
