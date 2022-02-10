package storing

import (
	"encoding/json"
	"time"

	"github.com/pkg/errors"
	"github.com/tidwall/buntdb"
	"nodemon/pkg/entities"
)

const (
	defaultRetentionDuration = 12 * time.Hour
)

const (
	stateHashIndexName = entities.NodeStatementStateHashJSONFieldName
)

type EventsStorage struct {
	db *buntdb.DB
}

func NewEventsStorage() (*EventsStorage, error) {
	db, err := buntdb.Open(":memory:")
	if err != nil {
		return nil, errors.Wrap(err, "failed to open events storage")
	}
	err = db.CreateIndex(stateHashIndexName, "*", buntdb.IndexJSON(entities.NodeStatementStateHashJSONFieldName))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create buntdb index %q", stateHashIndexName)
	}
	return &EventsStorage{db: db}, nil
}

func (s *EventsStorage) Close() error {
	if err := s.db.Close(); err != nil {
		return errors.Wrap(err, "failed to close events storage")
	}
	return nil
}

func (s *EventsStorage) GetStatement(nodeURL string, timestamp int64) (entities.NodeStatement, error) {
	var (
		value     string
		statement entities.NodeStatement
		key       = StatementKey{nodeURL, timestamp}.String()
	)
	err := s.db.View(func(tx *buntdb.Tx) error {
		var err error
		value, err = tx.Get(key)
		return err
	})
	if err != nil {
		return entities.NodeStatement{}, errors.Wrapf(err, "failed to get node statement from db by key %q", key)
	}
	if err := json.Unmarshal([]byte(value), &statement); err != nil {
		return entities.NodeStatement{}, errors.Wrap(err, "failed to unmarshal node statement")
	}
	return statement, nil
}

func (s *EventsStorage) PutEvent(event entities.Event) error {
	opts := &buntdb.SetOptions{Expires: true, TTL: defaultRetentionDuration}
	err := s.db.Update(func(tx *buntdb.Tx) error {
		v, err := s.makeValue(event)
		if err != nil {
			return err
		}
		key := StatementKey{event.Node(), event.Timestamp()}
		_, _, err = tx.Set(key.String(), v, opts)
		return err
	})
	if err != nil {
		return errors.Wrap(err, "failed to store event")
	}
	return nil
}

func (s *EventsStorage) StatementsCount() (int, error) {
	cnt := 0
	err := s.db.View(func(tx *buntdb.Tx) error {
		var err error
		cnt, err = tx.Len()
		return err
	})
	if err != nil {
		return 0, errors.Wrap(err, "failed to query statements")
	}
	return cnt, nil
}

func (s *EventsStorage) makeValue(e entities.Event) (string, error) {
	var v entities.NodeStatement
	switch te := e.(type) {
	case *entities.UnreachableEvent:
		v = entities.NodeStatement{
			Node:   e.Node(),
			Status: entities.Unreachable,
		}
	case *entities.VersionEvent:
		v = entities.NodeStatement{
			Node:    e.Node(),
			Status:  entities.Incomplete,
			Version: te.Version(),
		}
	case *entities.HeightEvent:
		v = entities.NodeStatement{
			Node:    e.Node(),
			Status:  entities.Incomplete,
			Version: te.Version(),
			Height:  te.Height(),
		}
	case *entities.InvalidHeightEvent:
		v = entities.NodeStatement{
			Node:    e.Node(),
			Status:  entities.InvalidVersion,
			Version: te.Version(),
			Height:  te.Height(),
		}
	case *entities.StateHashEvent:
		v = entities.NodeStatement{
			Node:      e.Node(),
			Status:    entities.OK,
			Version:   te.Version(),
			Height:    te.Height(),
			StateHash: te.StateHash(),
		}
	}
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
