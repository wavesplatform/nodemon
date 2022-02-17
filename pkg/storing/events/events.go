package events

import (
	"encoding/json"
	"strconv"
	"time"

	"github.com/pkg/errors"
	"github.com/tidwall/buntdb"
	"nodemon/pkg/entities"
)

type Storage struct {
	db                *buntdb.DB
	retentionDuration time.Duration
}

func NewStorage(retentionDuration time.Duration) (*Storage, error) {
	db, err := buntdb.Open(":memory:")
	if err != nil {
		return nil, errors.Wrap(err, "failed to open events storage")
	}
	return &Storage{db: db, retentionDuration: retentionDuration}, nil
}

func (s *Storage) Close() error {
	if err := s.db.Close(); err != nil {
		return errors.Wrap(err, "failed to close events storage")
	}
	return nil
}

func (s *Storage) GetStatement(nodeURL string, timestamp int64) (entities.NodeStatement, error) {
	var (
		value     string
		statement entities.NodeStatement
		key       = statementKey{nodeURL, timestamp}.String()
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

func (s *Storage) ViewStatementsByNodeURLWithDescendKeys(
	nodeURL string,
	iter func(*entities.NodeStatement) bool,
) error {
	pattern := newStatementKeyPattern(nodeURL, "*")
	if err := s.viewByKeyPatternWithDescendKeys(pattern, iter); err != nil {
		return errors.Wrapf(err, "failed to execute ViewStatementsByNodeURLWithDescendKeys for nodeURL %q", nodeURL)
	}
	return nil
}

func (s *Storage) ViewStatementsByTimestamp(
	timestamp int64,
	iter func(*entities.NodeStatement) bool,
) error {
	pattern := newStatementKeyPattern("*", strconv.FormatInt(timestamp, 10))
	if err := s.viewByKeyPatternWithDescendKeys(pattern, iter); err != nil {
		return errors.Wrapf(err, "failed to execute ViewStatementsByTimestamp for timestamp %d", timestamp)
	}
	return nil
}

func (s *Storage) viewByKeyPatternWithDescendKeys(
	pattern string,
	iter func(*entities.NodeStatement) bool,
) error {
	return s.db.View(func(tx *buntdb.Tx) (err error) {
		var (
			unmarshalErr error
		)
		defer func() {
			isUnmarshalErr := unmarshalErr != nil
			switch {
			case err != nil && isUnmarshalErr: // dbErr != nil
				err = errors.Wrapf(err, "%v", unmarshalErr) // coupling two errors
			case isUnmarshalErr:
				err = unmarshalErr
			}
		}()
		dbErr := tx.DescendKeys(pattern, func(key, value string) bool {
			statement := new(entities.NodeStatement)
			if unmarshalErr = json.Unmarshal([]byte(value), statement); unmarshalErr != nil {
				unmarshalErr = errors.Wrapf(unmarshalErr, "failed to unmarshal NodeStatement by key %q", key)
				return false
			}
			return iter(statement)
		})
		return dbErr
	})
}

func (s *Storage) PutEvent(event entities.Event) error {
	opts := &buntdb.SetOptions{Expires: true, TTL: s.retentionDuration}
	v, err := s.makeValue(event)
	if err != nil {
		return err
	}
	key := statementKey{event.Node(), event.Timestamp()}.String()
	err = s.db.Update(func(tx *buntdb.Tx) error {
		var err error
		_, _, err = tx.Set(key, v, opts)
		return err
	})
	if err != nil {
		return errors.Wrap(err, "failed to store event")
	}
	return nil
}

func (s *Storage) StatementsCount() (int, error) {
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

func (s *Storage) makeValue(e entities.Event) (string, error) {
	v := e.Statement()
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
