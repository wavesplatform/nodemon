package criteria

import (
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

func (c *HeightCriterion) Analyze(alerts chan<- entities.Alert, timestamp int64, statements entities.NodeStatements) error {
	split := statements.SplitByNodeHeight()
	min, max := split.MinMaxHeight()
	if min == max { // all nodes on same height
		return nil
	}
	sortedMaxGroup := split[max].Nodes().Sort()
	for height, nodeStatements := range split {
		if diff := max - height; diff > c.opts.MaxHeightDiff {
			alerts <- &entities.HeightAlert{
				Timestamp: timestamp,
				MaxHeightGroup: entities.HeightGroup{
					Height: max,
					Nodes:  sortedMaxGroup,
				},
				OtherHeightGroup: entities.HeightGroup{
					Height: height,
					Nodes:  nodeStatements.Nodes().Sort(),
				},
			}
		}
	}
	return nil
}
