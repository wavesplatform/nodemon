package criteria

import (
	"fmt"

	"nodemon/pkg/entities"
	"nodemon/pkg/storing/events"
)

type InvalidHeightCriterion struct {
	es *events.Storage
}

func NewInvalidHeightCriterion(es *events.Storage) *InvalidHeightCriterion {
	return &InvalidHeightCriterion{es: es}
}

func (c *InvalidHeightCriterion) Analyze(alerts chan<- entities.Alert, statements entities.NodeStatements) error {
	for _, statement := range statements {
		if err := c.analyzeNodeStatement(alerts, statement); err != nil {
			return err
		}
	}
	return nil
}

func (c *InvalidHeightCriterion) analyzeNodeStatement(alerts chan<- entities.Alert, statement entities.NodeStatement) error {
	if statement.Status != entities.InvalidHeight { // fast path
		return nil
	}
	alerts <- entities.NewSimpleAlert(fmt.Sprintf("Node %q (%s) has invalid height %d",
		statement.Node, statement.Version, statement.Height,
	))
	return nil
}
