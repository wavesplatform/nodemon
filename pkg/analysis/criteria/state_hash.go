package criteria

import (
	"strings"

	"github.com/pkg/errors"
	"go.uber.org/zap"
	"nodemon/pkg/analysis/finders"
	"nodemon/pkg/entities"
	"nodemon/pkg/storing/events"
)

const (
	defaultMaxForkDepth     = 3
	defaultHeightBucketSize = 3
)

type StateHashCriterionOptions struct {
	MaxForkDepth     int
	HeightBucketSize int
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

func (c *StateHashCriterion) Analyze(alerts chan<- entities.Alert, timestamp int64, statements entities.NodeStatements) error {
	splitByBucketHeight := statements.SplitByNodeHeightBuckets(c.opts.HeightBucketSize)
	for bucketHeight, nodeStatements := range splitByBucketHeight {
		statementsAtBucketHeight := make(entities.NodeStatements, 0, len(nodeStatements))
		for _, statement := range nodeStatements {
			var statementAtBucketHeight entities.NodeStatement
			if statement.Height == bucketHeight {
				statementAtBucketHeight = statement
			} else {
				var err error
				statementAtBucketHeight, err = c.es.GetStatementAtHeight(statement.Node, bucketHeight)
				if err != nil {
					if errors.Is(err, events.NoFullStatementError) {
						c.zap.Sugar().Warnf("StateHashCriterion: No full statement for node %q at height %d",
							statement.Node, statement.Height,
						)
						continue
					}
					return errors.Wrapf(err, "failed to analyze statehash for nodes at bucketHeight=%d", bucketHeight)
				}
			}
			statementsAtBucketHeight = append(statementsAtBucketHeight, statementAtBucketHeight)
		}
		if err := c.analyzeNodesOnSameHeight(alerts, bucketHeight, timestamp, statementsAtBucketHeight); err != nil {
			return errors.Wrapf(err, "failed to analyze statehash for nodes at bucketHeight=%d", bucketHeight)
		}
	}
	return nil
}

func (c *StateHashCriterion) analyzeNodesOnSameHeight(
	alerts chan<- entities.Alert,
	bucketHeight int,
	timestamp int64,
	statements entities.NodeStatements,
) error {
	splitStateHash, others := statements.SplitBySumStateHash()
	if l := len(others); l != 0 {
		return errors.Errorf("failed to analyze nodes at height %d by statehash criterion %v",
			bucketHeight, others.Nodes(),
		)
	}
	if len(splitStateHash) < 2 { // same state hash
		return nil
	}
	// take sample from each state hash group
	samples := make(entities.NodeStatements, 0, len(splitStateHash))
	for _, nodeStatements := range splitStateHash {
		samples = append(samples, nodeStatements[0]) // group contains at least one node statement
	}
	samples.SortByNodeAsc() // sort for predictable alert result

	ff := finders.NewForkFinder(c.es)

	skip := make(map[string]struct{})
	for _, first := range samples {
		for _, second := range samples {
			if first.Node == second.Node {
				continue
			}
			skipKey := strings.Join(entities.Nodes{first.Node, second.Node}.Sort(), "")
			if _, in := skip[skipKey]; in {
				continue
			}
			lastCommonStateHashExist := true
			lastCommonStateHashHeight, lastCommonStateHash, err := ff.FindLastCommonStateHash(first.Node, second.Node)
			if err != nil {
				if errors.Is(err, finders.ErrNoCommonBlocks) {
					lastCommonStateHashExist = false
				} else {
					return errors.Wrapf(err, "failed to find last common state hash for nodes %q and %q",
						first.Node, second.Node,
					)
				}
			}
			forkDepth := bucketHeight - lastCommonStateHashHeight
			if forkDepth > c.opts.MaxForkDepth {
				skip[skipKey] = struct{}{}
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
		}
	}
	return nil
}
