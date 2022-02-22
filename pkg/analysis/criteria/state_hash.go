package criteria

import (
	"fmt"

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

func (c *StateHashCriterion) Analyze(alerts chan<- entities.Alert, statements entities.NodeStatements) error {
	splitHeight := statements.SplitByNodeHeight()
	for height, nodeStatements := range splitHeight {
		if err := c.analyzeNodesOnSameHeight(alerts, height, nodeStatements); err != nil {
			return err
		}
	}
	return nil
}

func (c *StateHashCriterion) analyzeNodesOnSameHeight(alerts chan<- entities.Alert, groupHeight int, statements entities.NodeStatements) error {
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

	for _, first := range samples {
		for _, second := range samples {
			if first.Node == second.Node {
				continue
			}
			forkHeight, commonStateHash, err := ff.FindLastCommonStateHash(first.Node, second.Node)
			if err != nil {
				return errors.Wrapf(err, "failed to find last common state hash for nodes %q and %q",
					first.Node, second.Node,
				)
			}
			if groupHeight-forkHeight > c.opts.MaxForkDepth {
				alerts <- entities.NewSimpleAlert(fmt.Sprintf(
					"Different state hash between nodes on same height %d: %q=%v, %q=%v. Fork occured after: height %d, statehash %q, blockID %q",
					groupHeight,
					first.StateHash.SumHash.String(),
					splitStateHash[first.StateHash.SumHash].Nodes().Sort(),
					second.StateHash.SumHash.String(),
					splitStateHash[second.StateHash.SumHash].Nodes().Sort(),
					forkHeight,
					commonStateHash.SumHash.String(),
					commonStateHash.BlockID.String(),
				))
			}
		}
	}
	return nil
}
