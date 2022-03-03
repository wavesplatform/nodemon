package criteria

import (
	"strings"

	"github.com/pkg/errors"
	"nodemon/pkg/analysis/finders"
	"nodemon/pkg/entities"
	"nodemon/pkg/storing/events"
)

type StateHashCriterionOptions struct {
	MaxForkDepth int
}

type StateHashCriterion struct {
	opts *StateHashCriterionOptions
	es   *events.Storage
}

func NewStateHashCriterion(es *events.Storage, opts *StateHashCriterionOptions) *StateHashCriterion {
	if opts == nil { // default
		opts = &StateHashCriterionOptions{
			MaxForkDepth: 5,
		}
	}
	return &StateHashCriterion{opts: opts, es: es}
}

func (c *StateHashCriterion) Analyze(alerts chan<- entities.Alert, timestamp int64, statements entities.NodeStatements) error {
	splitHeight := statements.SplitByNodeHeight()
	for height, nodeStatements := range splitHeight {
		if err := c.analyzeNodesOnSameHeight(alerts, height, timestamp, nodeStatements); err != nil {
			return err
		}
	}
	return nil
}

func (c *StateHashCriterion) analyzeNodesOnSameHeight(
	alerts chan<- entities.Alert,
	groupHeight int,
	timestamp int64,
	statements entities.NodeStatements,
) error {
	splitStateHash, others := statements.SplitBySumStateHash()
	if l := len(others); l != 0 {
		return errors.Errorf("failed to analyze nodes at height %d by statehash criterion %v",
			groupHeight, others.Nodes(),
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
			lastCommonStateHashHeight, lastCommonStateHash, err := ff.FindLastCommonStateHash(first.Node, second.Node)
			if err != nil {
				return errors.Wrapf(err, "failed to find last common state hash for nodes %q and %q",
					first.Node, second.Node,
				)
			}
			if groupHeight-lastCommonStateHashHeight > c.opts.MaxForkDepth {
				skip[skipKey] = struct{}{}
				alerts <- &entities.StateHashAlert{
					Timestamp:                 timestamp,
					CurrentGroupsHeight:       groupHeight,
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
