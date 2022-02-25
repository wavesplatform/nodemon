package criteria

import (
	"nodemon/pkg/entities"
	"nodemon/pkg/storing/events"
)

type IncompleteCriterion struct {
	es *events.Storage
}

func NewIncompleteCriterion(es *events.Storage) *IncompleteCriterion {
	return &IncompleteCriterion{es: es}
}

func (c *IncompleteCriterion) Analyze(alerts chan<- entities.Alert, statements entities.NodeStatements) error {
	for _, statement := range statements {
		if err := c.analyzeNodeStatement(alerts, statement); err != nil {
			return err
		}
	}
	return nil
}

func (c *IncompleteCriterion) analyzeNodeStatement(alerts chan<- entities.Alert, statement entities.NodeStatement) error {
	if statement.Status != entities.Incomplete { // fast path
		return nil
	}
	alerts <- &entities.IncompleteAlert{NodeStatement: statement}
	return nil
}
