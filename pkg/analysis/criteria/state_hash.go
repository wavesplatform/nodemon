package criteria

import (
	"strings"

	"nodemon/pkg/analysis/finders"
	"nodemon/pkg/entities"
	"nodemon/pkg/storing/events"

	"github.com/pkg/errors"
	"go.uber.org/zap"
)

const (
	defaultMaxForkDepth     = 3
	defaultHeightBucketSize = 3
)

type StateHashCriterionOptions struct {
	MaxForkDepth     uint32
	HeightBucketSize uint32
}

type StateHashCriterion struct {
	opts *StateHashCriterionOptions
	es   *events.Storage
	zap  *zap.Logger
}

func NewStateHashCriterion(es *events.Storage, opts *StateHashCriterionOptions, zap *zap.Logger) *StateHashCriterion {
	if opts == nil { // default
		opts = &StateHashCriterionOptions{
			MaxForkDepth:     defaultMaxForkDepth,
			HeightBucketSize: defaultHeightBucketSize,
		}
	}
	return &StateHashCriterion{opts: opts, es: es, zap: zap}
}

func (c *StateHashCriterion) Analyze(alerts chan<- entities.Alert, ts int64, statements entities.NodeStatements) error {
	splitByBucketHeight := statements.SplitByNodeHeightBuckets(uint64(c.opts.HeightBucketSize))
	for bucketHeight, nodeStatements := range splitByBucketHeight {
		var statementsAtBucketHeight entities.NodeStatements
		if minHeight, maxHeight := nodeStatements.SplitByNodeHeight().MinMaxHeight(); minHeight == maxHeight {
			// all nodes are on the same height
			bucketHeight = minHeight
			statementsAtBucketHeight = nodeStatements
		} else {
			var err error
			statementsAtBucketHeight, err = c.getAllStatementsAtBucketHeight(nodeStatements, bucketHeight)
			if err != nil {
				return err
			}
		}
		if err := c.analyzeNodesOnSameHeight(alerts, bucketHeight, ts, statementsAtBucketHeight); err != nil {
			return errors.Wrapf(err, "failed to analyze statehash for nodes at bucketHeight=%d", bucketHeight)
		}
	}
	return nil
}

func (c *StateHashCriterion) getAllStatementsAtBucketHeight(
	nodeStatements entities.NodeStatements,
	bucketHeight uint64,
) (entities.NodeStatements, error) {
	statementsAtBucketHeight := make(entities.NodeStatements, 0, len(nodeStatements))
	for _, statement := range nodeStatements {
		var statementAtBucketHeight entities.NodeStatement
		if statement.Height == bucketHeight {
			statementAtBucketHeight = statement
		} else {
			var err error
			statementAtBucketHeight, err = c.es.GetFullStatementAtHeight(statement.Node, bucketHeight)
			if err != nil {
				if !errors.Is(err, events.ErrNoFullStatement) {
					return nil, errors.Wrapf(err, "failed to analyze statehash for nodes at bucketHeight=%d",
						bucketHeight,
					)
				}
				c.zap.Sugar().Warnf(
					"StateHashCriterion: No full statement for node %q with height %d at bucketHeight %d: %v",
					statement.Node, statement.Height, bucketHeight, err,
				)
				continue
			}
		}
		statementsAtBucketHeight = append(statementsAtBucketHeight, statementAtBucketHeight)
	}
	return statementsAtBucketHeight, nil
}

func (c *StateHashCriterion) analyzeNodesOnSameHeight(
	alerts chan<- entities.Alert,
	bucketHeight uint64,
	timestamp int64,
	statements entities.NodeStatements,
) error {
	splitStateHash, others := statements.SplitBySumStateHash()
	if l := len(others); l != 0 {
		return errors.Errorf("failed to analyze nodes at height %d by statehash criterion %v",
			bucketHeight, others.Nodes(),
		)
	}
	if len(splitStateHash) <= 1 { // same state hash
		return nil
	}
	// take sample from each state hash group
	samples := make(entities.NodeStatements, 0, len(splitStateHash))
	for _, nodeStatements := range splitStateHash {
		samples = append(samples, nodeStatements[0]) // group contains at least one node statement
	}
	samples.SortByNodeAsc() // sort for predictable alert result

	ff := finders.NewForkFinder(c.es).WithLinearSearchParams(uint64(c.opts.MaxForkDepth + 1))

	for i, first := range samples {
		for _, second := range samples[i+1:] {
			err := c.handleSamplesPair(alerts, bucketHeight, timestamp, ff, first, second, splitStateHash)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (c *StateHashCriterion) handleSamplesPair(
	alerts chan<- entities.Alert,
	bucketHeight uint64,
	timestamp int64,
	ff *finders.ForkFinder,
	first entities.NodeStatement,
	second entities.NodeStatement,
	splitStateHash entities.NodeStatementsSplitByStateHash,
) error {
	lastCommonStateHashExist := true
	lastCommonStateHashHeight, lastCommonStateHash, err := ff.FindLastCommonStateHash(first.Node, second.Node)
	if err != nil {
		switch {
		case errors.Is(err, finders.ErrNoFullStatement):
			c.zap.Sugar().Warnf(
				"StateHashCriterion: Failed to find last common state hash for nodes %q and %q: %v",
				first.Node, second.Node, err,
			)
			return nil
		case errors.Is(err, finders.ErrNoCommonBlocks):
			lastCommonStateHashExist = false
			c.zap.Sugar().Warnf(
				"StateHashCriterion: Failed to find last common state hash for nodes %q and %q: %v",
				first.Node, second.Node, err,
			)
		default:
			return errors.Wrapf(err, "failed to find last common state hash for nodes %q and %q",
				first.Node, second.Node,
			)
		}
	}
	forkDepth := int64(bucketHeight - lastCommonStateHashHeight) //#nosec: height can't be negative, but the diff can

	if forkDepth != 0 {
		msg := "StateHashCriterion: fork detected"
		if forkDepth < 0 {
			msg = "StateHashCriterion: last common StateHash height is greater than bucket height"
		}
		c.zap.Info(msg,
			zap.Int64("Fork depth", forkDepth),
			zap.Bool("Last common StateHash exist", lastCommonStateHashExist),
			zap.Uint64("Bucket height", bucketHeight),
			zap.Uint64("Last common StateHash height", lastCommonStateHashHeight),
			zap.String("First group",
				strings.Join(splitStateHash[first.StateHash.SumHash].Nodes().Sort(), ", "),
			),
			zap.String("First group StateHash", first.StateHash.SumHash.Hex()),
			zap.String("Second group", strings.Join(
				splitStateHash[second.StateHash.SumHash].Nodes().Sort(), ", "),
			),
			zap.String("Second group StateHash", second.StateHash.SumHash.Hex()))
	}

	if forkDepth > int64(c.opts.MaxForkDepth) || forkDepth >= int64(c.opts.HeightBucketSize) {
		alerts <- &entities.StateHashAlert{
			Timestamp:                 timestamp,
			CurrentGroupsBucketHeight: bucketHeight,
			LastCommonStateHashExist:  lastCommonStateHashExist,
			LastCommonStateHashHeight: lastCommonStateHashHeight,
			LastCommonStateHash:       lastCommonStateHash,
			FirstGroup: entities.StateHashGroup{
				Nodes:     splitStateHash[first.StateHash.SumHash].Nodes().Sort(),
				StateHash: *first.StateHash,
			},
			SecondGroup: entities.StateHashGroup{
				Nodes:     splitStateHash[second.StateHash.SumHash].Nodes().Sort(),
				StateHash: *second.StateHash,
			},
		}
	}
	return nil
}
