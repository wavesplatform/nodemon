package criteria

import (
	"fmt"
	"time"

	"github.com/pkg/errors"
	"nodemon/pkg/entities"
	"nodemon/pkg/storing/events"
)

type UnreachableCriterionOptions struct {
	Streak int
	Depth  int
}

type UnreachableCriterion struct {
	opts *UnreachableCriterionOptions
	es   *events.Storage
}

func NewUnreachableCriterion(es *events.Storage, opts *UnreachableCriterionOptions) *UnreachableCriterion {
	if opts == nil { // by default
		opts = &UnreachableCriterionOptions{
			Streak: 3,
			Depth:  10,
		}
	}
	return &UnreachableCriterion{opts: opts, es: es}
}

func (c *UnreachableCriterion) Analyze(alerts chan<- entities.Alert, statements entities.NodeStatements) error {
	for _, statement := range statements {
		if err := c.analyzeNodeStatement(alerts, statement); err != nil {
			return err
		}
	}
	return nil
}

func (c *UnreachableCriterion) analyzeNodeStatement(alerts chan<- entities.Alert, statement entities.NodeStatement) error {
	var (
		streak                 = 0
		depth                  = 0
		streakStart, streakEnd *entities.NodeStatement
	)
	err := c.es.ViewStatementsByNodeWithDescendKeys(statement.Node, func(statement *entities.NodeStatement) bool {
		switch statement.Status {
		case entities.Unreachable:
			if streak == 0 {
				streakEnd = statement
			}
			streak += 1
			streakStart = statement
		default:
			streak = 0
			streakEnd, streakStart = nil, nil
		}
		depth++
		if streak >= c.opts.Streak || depth > c.opts.Depth {
			return false
		}
		return true
	})
	if err != nil {
		return errors.Wrapf(err, "failed to analyze %q by unreachable criterion", statement.Node)
	}
	if streak >= c.opts.Streak {
		// TODO(nickeskov): create alert type for this criterion
		alerts <- entities.NewSimpleAlert(fmt.Sprintf("Node %q (%s) is UNREACHABLE since %q to %q",
			statement.Node, statement.Version,
			time.Unix(streakStart.Timestamp, 0), time.Unix(streakEnd.Timestamp, 0),
		))
	}
	return nil
}
