package criteria

import (
	"nodemon/pkg/entities"

	"github.com/pkg/errors"
)

type BaseTargetCriterionOptions struct {
	Threshold int
}

type BaseTargetCriterion struct {
	opts *BaseTargetCriterionOptions
}

func NewBaseTargetCriterion(opts *BaseTargetCriterionOptions) (*BaseTargetCriterion, error) {
	if opts == nil {
		return nil, errors.New("provided <nil> base target criterion options")
	}
	return &BaseTargetCriterion{opts: opts}, nil
}

func (c *BaseTargetCriterion) Analyze(alerts chan<- entities.Alert, ts int64, statements entities.NodeStatements) {
	baseTarget := mostFrequentBaseTarget(statements)
	if baseTarget <= c.opts.Threshold {
		return // it's ok, nothing to do
	}
	baseTargetValues := make([]entities.BaseTargetValue, 0, len(statements))
	for _, nodeStatement := range statements {
		baseTargetValues = append(baseTargetValues, entities.BaseTargetValue{
			Node:       nodeStatement.Node,
			BaseTarget: nodeStatement.BaseTarget,
		})
	}
	alerts <- &entities.BaseTargetAlert{
		Timestamp:        ts,
		BaseTargetValues: baseTargetValues,
		Threshold:        c.opts.Threshold,
	}
}

func mostFrequentBaseTarget(statements entities.NodeStatements) int {
	m := make(map[int]int)
	var max int
	var maxKey int
	for _, s := range statements {
		if s.BaseTarget == 0 {
			continue
		}
		v := s.BaseTarget
		m[v]++
		if m[v] > max {
			max = m[v]
			maxKey = v
		}
	}
	return maxKey
}
