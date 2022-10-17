package criteria

import (
	"nodemon/pkg/entities"
)

type BaseTargetCriterionOptions struct {
	Threshold int64
}

type BaseTargetCriterion struct {
	opts *BaseTargetCriterionOptions
}

func NewBaseTargetCriterion(opts *BaseTargetCriterionOptions) *BaseTargetCriterion {
	if opts == nil { // default
		opts = &BaseTargetCriterionOptions{
			Threshold: opts.Threshold,
		}
	}
	return &BaseTargetCriterion{opts: opts}
}

func (c *BaseTargetCriterion) Analyze(alerts chan<- entities.Alert, timestamp int64, statements entities.NodeStatements) {
	var baseTargetValues []entities.BaseTargetValue
	shouldAlert := false
	for _, nodeStatement := range statements {
		baseTargetValues = append(baseTargetValues, entities.BaseTargetValue{Node: nodeStatement.Node, BaseTarget: nodeStatement.BaseTarget})
		if nodeStatement.BaseTarget >= c.opts.Threshold {
			shouldAlert = true
		}
	}

	if shouldAlert {
		alerts <- &entities.BaseTargetAlert{Timestamp: timestamp, BaseTargetValues: baseTargetValues, Threshold: c.opts.Threshold}
	}
}
