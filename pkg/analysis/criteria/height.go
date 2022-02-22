package criteria

import (
	"fmt"

	"nodemon/pkg/entities"
	"nodemon/pkg/storing/events"
)

type HeightCriterionOptions struct {
	MaxHeightDiff int
}

type HeightCriterion struct {
	opts *HeightCriterionOptions
	es   *events.Storage
}

func NewHeightCriterion(es *events.Storage, opts *HeightCriterionOptions) *HeightCriterion {
	if opts == nil { // default
		opts = &HeightCriterionOptions{
			MaxHeightDiff: 20,
		}
	}
	return &HeightCriterion{opts: opts, es: es}
}

func (c *HeightCriterion) Analyze(alerts chan<- entities.Alert, statements entities.NodeStatements) error {
	split := statements.Iterator().SplitByNodeHeight()
	min, max := split.MinMaxHeight()
	if min == max { // all nodes on same height
		return nil
	}
	sortedMaxGroup := split[max].Nodes().Sort()
	for height, nodeStatements := range split {
		if diff := max - height; diff > c.opts.MaxHeightDiff {
			// TODO(nickeskov): create alert type for this criterion
			alerts <- entities.NewSimpleAlert(fmt.Sprintf(
				"Too big height (%d - %d = %d > %d) diff between nodes groups: max=%v, other=%v",
				max, height, diff, c.opts.MaxHeightDiff,
				sortedMaxGroup, nodeStatements.Nodes().Sort(),
			))
		}
	}
	return nil
}
