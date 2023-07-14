package criteria

import (
	"nodemon/pkg/entities"
	"nodemon/pkg/storing/events"

	"github.com/pkg/errors"
	"go.uber.org/zap"
)

type UnreachableCriterionOptions struct {
	Streak int
	Depth  int
}

type UnreachableCriterion struct {
	opts *UnreachableCriterionOptions
	es   *events.Storage
	zap  *zap.Logger
}

const (
	unreachableStreakDefault = 3
	unreachableDepthDefault  = 5
)

func NewUnreachableCriterion(
	es *events.Storage,
	opts *UnreachableCriterionOptions,
	logger *zap.Logger,
) *UnreachableCriterion {
	if opts == nil { // by default
		opts = &UnreachableCriterionOptions{
			Streak: unreachableStreakDefault,
			Depth:  unreachableDepthDefault,
		}
	}
	return &UnreachableCriterion{opts: opts, es: es, zap: logger}
}

func (c *UnreachableCriterion) Analyze(
	alerts chan<- entities.Alert,
	timestamp int64,
	statements entities.NodeStatements,
) error {
	for _, statement := range statements {
		if err := c.analyzeNode(alerts, timestamp, statement.Node); err != nil {
			return err
		}
	}
	return nil
}

func (c *UnreachableCriterion) analyzeNode(alerts chan<- entities.Alert, timestamp int64, node string) error {
	var (
		streak = 0
		depth  = 0
	)
	err := c.es.ViewStatementsByNodeWithDescendKeys(node, func(statement *entities.NodeStatement) bool {
		if statement.Status == entities.Unreachable {
			streak++
		} else {
			streak = 0
		}
		depth++
		if streak >= c.opts.Streak || depth >= c.opts.Depth {
			return false
		}
		return true
	})
	if err != nil {
		return errors.Wrapf(err, "failed to analyze %q by unreachable criterion", node)
	}
	if streak > 0 {
		c.zap.Info("UnreachableCriterion: unreachable statement",
			zap.String("node", node), zap.Int("streak", streak),
		)
	}
	if streak >= c.opts.Streak {
		alerts <- &entities.UnreachableAlert{
			Node:      node,
			Timestamp: timestamp,
		}
	}
	return nil
}
