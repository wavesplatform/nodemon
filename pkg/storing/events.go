package storing

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/pkg/errors"
	"github.com/tidwall/buntdb"
	"nodemon/pkg/entities"
)

const (
	defaultRetentionDuration = 12 * time.Hour
)

type EventsStorage struct {
	db *buntdb.DB
}

func NewEventsStorage() (*EventsStorage, error) {
	db, err := buntdb.Open(":memory:")
	if err != nil {
		return nil, errors.Wrap(err, "failed to open events storage")
	}
	return &EventsStorage{db: db}, nil
}

func (s *EventsStorage) Close() error {
	if err := s.db.Close(); err != nil {
		return errors.Wrap(err, "failed to close events storage")
	}
	return nil
}

func (s *EventsStorage) PutEvent(event entities.Event) error {
	opts := &buntdb.SetOptions{Expires: true, TTL: defaultRetentionDuration}
	err := s.db.Update(func(tx *buntdb.Tx) error {
		v, err := s.makeValue(event)
		if err != nil {
			return err
		}
		_, _, err = tx.Set(s.key(event), v, opts)
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
		err := tx.Ascend("", func(key, value string) bool {
			cnt++
			return true
		})
		return err
	})
	if err != nil {
		return 0, errors.Wrap(err, "failed to query statements")
	}
	return cnt, nil
}

func (s *EventsStorage) key(e entities.Event) string {
	return fmt.Sprintf("node:%s:ts:%d", e.Node(), e.Timestamp())
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
