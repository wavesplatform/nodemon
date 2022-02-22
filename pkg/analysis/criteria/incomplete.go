package criteria

import (
	"encoding/json"
	"fmt"

	"github.com/pkg/errors"
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
	v, err := json.Marshal(&statement)
	if err != nil {
		return errors.Wrap(err, "failed to analyzeNodeStatement by IncompleteCriterion")
	}
	alerts <- entities.NewSimpleAlert(fmt.Sprintf("Node %q (%s) has incomplete statement info %q",
		statement.Node, statement.Version, string(v),
	))
	return nil
}
