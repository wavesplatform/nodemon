package criteria

import (
	"nodemon/pkg/entities"

	"github.com/pkg/errors"
)

type BaseTargetCriterionOptions struct {
	Threshold uint64
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

func mostFrequentBaseTarget(statements entities.NodeStatements) uint64 {
	m := make(map[uint64]uint64)
	var maxBT uint64
	var maxKey uint64
	for _, s := range statements {
		if s.BaseTarget == 0 {
			continue
		}
		v := s.BaseTarget
		m[v]++
		if m[v] > maxBT {
			maxBT = m[v]
			maxKey = v
		}
	}
	return maxKey
}
