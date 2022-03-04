package criteria

import (
	"nodemon/pkg/entities"
)

type HeightCriterionOptions struct {
	MaxHeightDiff int
}

type HeightCriterion struct {
	opts *HeightCriterionOptions
}

func NewHeightCriterion(opts *HeightCriterionOptions) *HeightCriterion {
	if opts == nil { // default
		opts = &HeightCriterionOptions{
			MaxHeightDiff: 20,
		}
	}
	return &HeightCriterion{opts: opts}
}

func (c *HeightCriterion) Analyze(alerts chan<- entities.Alert, timestamp int64, statements entities.NodeStatements) {
	split := statements.SplitByNodeHeight()
	min, max := split.MinMaxHeight()
	if min == max { // all nodes on same height
		return
	}
	sortedMaxGroup := split[max].Nodes().Sort()
	for height, nodeStatements := range split {
		if max-height > c.opts.MaxHeightDiff {
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
}
