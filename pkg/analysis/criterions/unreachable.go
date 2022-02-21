package criterions

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
	options *UnreachableCriterionOptions
	es      *events.Storage
}

func NewUnreachableCriterion(
	es *events.Storage,
	opts *UnreachableCriterionOptions,
) *UnreachableCriterion {
	if opts == nil {
		// by default
		opts = &UnreachableCriterionOptions{
			Streak: 3,
			Depth:  10,
		}
	}
	return &UnreachableCriterion{
		options: opts,
		es:      es,
	}
}

func (u *UnreachableCriterion) Analyze(alerts chan<- entities.Alert, statements entities.NodeStatements) error {
	for _, statement := range statements {
		if err := u.analyzeNodeStatement(alerts, statement); err != nil {
			return err
		}
	}
	return nil
}

func (u *UnreachableCriterion) analyzeNodeStatement(alerts chan<- entities.Alert, statement entities.NodeStatement) error {
	var (
		streak                 = 0
		depth                  = 0
		streakStart, streakEnd *entities.NodeStatement
	)
	err := u.es.ViewStatementsByNodeWithDescendKeys(statement.Node, func(statement *entities.NodeStatement) bool {
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
		if streak >= u.options.Streak || depth > u.options.Depth {
			return false
		}
		return true
	})
	if err != nil {
		return errors.Wrapf(err, "failed to analyze %q by unreachable criterion", statement.Node)
	}
	if streak >= u.options.Streak {
		alerts <- entities.NewSimpleAlert(fmt.Sprintf("Node %q is UNREACHABLE since %q to %q",
			statement.Node, time.Unix(streakStart.Timestamp, 0), time.Unix(streakEnd.Timestamp, 0),
		))
	}
	return nil
}
