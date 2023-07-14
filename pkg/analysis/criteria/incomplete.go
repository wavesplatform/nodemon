package criteria

import (
	"nodemon/pkg/entities"
	"nodemon/pkg/storing/events"

	"github.com/pkg/errors"
	"go.uber.org/zap"
)

type IncompleteCriterionOptions struct {
	Streak                              int
	Depth                               int
	ConsiderPrevUnreachableAsIncomplete bool
}

type IncompleteCriterion struct {
	opts *IncompleteCriterionOptions
	es   *events.Storage
	zap  *zap.Logger
}

const (
	incompleteStreakDefault                              = 3
	incompleteDepthDefault                               = 5
	incompleteConsiderPrevUnreachableAsIncompleteDefault = true
)

func NewIncompleteCriterion(
	es *events.Storage,
	opts *IncompleteCriterionOptions,
	logger *zap.Logger,
) *IncompleteCriterion {
	if opts == nil { // by default
		opts = &IncompleteCriterionOptions{
			Streak:                              incompleteStreakDefault,
			Depth:                               incompleteDepthDefault,
			ConsiderPrevUnreachableAsIncomplete: incompleteConsiderPrevUnreachableAsIncompleteDefault,
		}
	}
	return &IncompleteCriterion{opts: opts, es: es, zap: logger}
}

func (c *IncompleteCriterion) Analyze(alerts chan<- entities.Alert, statements entities.NodeStatements) error {
	for _, statement := range statements {
		if err := c.analyzeNode(alerts, statement); err != nil {
			return err
		}
	}
	return nil
}

func (c *IncompleteCriterion) analyzeNode(alerts chan<- entities.Alert, statement entities.NodeStatement) error {
	var (
		streak = 0
		depth  = 0
	)
	err := c.es.ViewStatementsByNodeWithDescendKeys(statement.Node, func(statement *entities.NodeStatement) bool {
		s := statement.Status
		if s == entities.Incomplete || (c.opts.ConsiderPrevUnreachableAsIncomplete && s == entities.Unreachable) {
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
		return errors.Wrapf(err, "failed to analyze %q by incomplete criterion", statement.Node)
	}
	if streak > 0 {
		c.zap.Info("IncompleteCriterion: incomplete statement", zap.String("node", statement.Node), zap.Int("streak", streak))
	}
	if streak >= c.opts.Streak {
		alerts <- &entities.IncompleteAlert{NodeStatement: statement}
	}
	return nil
}
