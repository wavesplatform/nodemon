package events

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"strconv"
	"time"

	"github.com/pkg/errors"
	"github.com/tidwall/buntdb"
	"github.com/wavesplatform/gowaves/pkg/proto"
	"nodemon/pkg/entities"
)

var (
	ErrNotFound = buntdb.ErrNotFound
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

func (s *Storage) ViewStatementsByNodeWithDescendKeys(node string, iter func(*entities.NodeStatement) bool) error {
	pattern := newStatementKey(node, "*")
	if err := s.viewByKeyPatternWithDescendKeys(pattern, iter); err != nil {
		return errors.Wrapf(err, "failed to execute ViewStatementsByNodeWithDescendKeys for node %q", node)
	}
	return nil
}

func (s *Storage) ViewStatementsByTimestamp(timestamp int64, iter func(*entities.NodeStatement) bool) error {
	pattern := newStatementKey("*", strconv.FormatInt(timestamp, 10))
	if err := s.viewByKeyPatternWithDescendKeys(pattern, iter); err != nil {
		return errors.Wrapf(err, "failed to execute ViewStatementsByTimestamp for timestamp %d", timestamp)
	}
	return nil
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
	log.Printf("New statement for node %s:\n%s\n", event.Node(), v)
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

func (s *Storage) EarliestHeight(node string) (int, error) {
	pattern := newStatementKey(node, "*")
	var h int
	err := s.viewByKeyPatternWithAscendKeys(pattern, func(s *entities.NodeStatement) bool {
		if s.StateHash != nil {
			h = s.Height
			return false
		}
		return true
	})
	if err != nil {
		return 0, errors.Wrapf(err, "failed to get earliest height for node '%s'", node)
	}
	if h == 0 {
		return 0, errors.Errorf("no full statements for node '%s'", node)
	}
	return h, nil
}

func (s *Storage) LatestHeight(node string) (int, error) {
	pattern := newStatementKey(node, "*")
	var h int
	err := s.viewByKeyPatternWithDescendKeys(pattern, func(s *entities.NodeStatement) bool {
		if s.StateHash != nil {
			h = s.Height
			return false
		}
		return true
	})
	if err != nil {
		return 0, errors.Wrapf(err, "failed to get latest height for node '%s'", node)
	}
	if h == 0 {
		return 0, errors.Errorf("no full statements for node '%s'", node)
	}
	return h, nil
}

var BigHeightDifference = errors.New("The height difference between nodes is more than 10")
var StorageIsNotReady = errors.New("The storage has not collected enough statements for status")

func (s *Storage) FindAllStatehashesOnCommonHeight(nodes []string) ([]entities.NodeStatement, error) {
	var heightDifference = 10

	minHeight, maxHeight := math.MaxInt, 0

	nodesList := make(map[string]bool) // reachable or unreachable
	for _, node := range nodes {
		nodesList[node] = true
	}

	// looking for the min common height and the max height
	var nodesHeights []entities.NodeStatement
	for node := range nodesList {
		h, err := s.LatestHeight(node)
		if err != nil || h == 0 {
			nodesList[node] = false // this node is unreachable
			nodesHeights = append(nodesHeights, entities.NodeStatement{Node: node, Status: entities.Unreachable})
			continue
		}
		if h < minHeight {
			minHeight = h
		}

		if h > maxHeight {
			maxHeight = h
		}
		nodesHeights = append(nodesHeights, entities.NodeStatement{Node: node, Height: h, Status: entities.OK})
	}

	if (maxHeight - minHeight) > heightDifference {
		return nodesHeights, BigHeightDifference
	}
	var statementsOnHeight []entities.NodeStatement
	for node, reachable := range nodesList {
		if !reachable {
			statementsOnHeight = append(statementsOnHeight, entities.NodeStatement{Node: node, Status: entities.Unreachable})
			continue
		}
		var foundStatementOnHeight bool
		err := s.ViewStatementsByNodeWithDescendKeys(node, func(statement *entities.NodeStatement) bool {
			// iterator. if true then continue
			if statement.Status != entities.OK {
				return true
			}
			if statement.Height == minHeight {
				statementsOnHeight = append(statementsOnHeight, *statement)
				foundStatementOnHeight = true
				return false
			}
			if statement.Height < minHeight { // more than 10 blocks were iterated, but not found the right block
				return false
			}
			return true
		})
		if err != nil {
			return nil, err
		}

		if !foundStatementOnHeight {
			fmt.Printf("NOT FOUND STATEMENT FOR NODE %s\n", node)
			return nil, StorageIsNotReady
		}

	}

	return statementsOnHeight, nil
}

func (s *Storage) LastStateHashAtHeight(node string, height int) (proto.StateHash, error) {
	pattern := newStatementKey(node, "*")
	var sh *proto.StateHash
	err := s.viewByKeyPatternWithDescendKeys(pattern, func(s *entities.NodeStatement) bool {
		st := *s
		if st.Height != height {
			return true
		}
		if st.StateHash != nil {
			sh = st.StateHash
			return false
		}
		return true
	})
	if err != nil {
		return proto.StateHash{}, errors.Wrapf(err, "failed to get the last state hash at height %d for node '%s'", height, node)
	}
	if sh == nil {
		return proto.StateHash{}, errors.Errorf("no full statements at height %d for node '%s'", height, node)
	}
	return *sh, nil
}

func (s *Storage) viewByKeyPatternWithDescendKeys(pattern string, iter func(*entities.NodeStatement) bool) error {
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

func (s *Storage) viewByKeyPatternWithAscendKeys(pattern string, iter func(*entities.NodeStatement) bool) error {
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
		dbErr := tx.AscendKeys(pattern, func(key, value string) bool {
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

func (s *Storage) makeValue(e entities.Event) (string, error) {
	v := e.Statement()
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
