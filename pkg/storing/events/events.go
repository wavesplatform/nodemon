package events

import (
	"encoding/json"
	"math"
	"strconv"
	"time"

	"nodemon/pkg/entities"

	"github.com/pkg/errors"
	"github.com/tidwall/buntdb"
	"github.com/wavesplatform/gowaves/pkg/proto"
	"go.uber.org/zap"
)

var (
	ErrNotFound = buntdb.ErrNotFound
)

const (
	depthCommonHeightSearch = 50
	heightDifference        = 50
)

type Storage struct {
	db                *buntdb.DB
	retentionDuration time.Duration
	zap               *zap.Logger
}

func NewStorage(retentionDuration time.Duration, logger *zap.Logger) (*Storage, error) {
	db, err := buntdb.Open(":memory:")
	if err != nil {
		return nil, errors.Wrap(err, "failed to open events storage")
	}
	return &Storage{db: db, retentionDuration: retentionDuration, zap: logger}, nil
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
	if unmarshalErr := json.Unmarshal([]byte(value), &statement); unmarshalErr != nil {
		return entities.NodeStatement{}, errors.Wrap(unmarshalErr, "failed to unmarshal node statement")
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
		var setErr error
		_, _, setErr = tx.Set(key, v, opts)
		return setErr
	})
	if err != nil {
		return errors.Wrap(err, "failed to store event")
	}
	s.zap.Debug("New statement for node", zap.String("node", event.Node()), zap.String("statement", v))
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

func (s *Storage) EarliestHeight(node string) (uint64, error) {
	pattern := newStatementKey(node, "*")
	var h uint64
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
		return 0, errors.Wrapf(ErrNoFullStatement, "no full statement at earliest height for node '%s'", node)
	}
	return h, nil
}

func (s *Storage) LatestHeight(node string) (uint64, error) {
	pattern := newStatementKey(node, "*")
	var h uint64
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
		return 0, errors.Wrapf(ErrNoFullStatement, "no full statement at latest height for node '%s'", node)
	}
	return h, nil
}

func (s *Storage) GetFullStatementAtHeight(node string, height uint64) (entities.NodeStatement, error) {
	pattern := newStatementKey(node, "*")
	var (
		st           = entities.NodeStatement{}
		notFound     = true
		notFoundFull = true
	)
	err := s.viewByKeyPatternWithDescendKeys(pattern, func(s *entities.NodeStatement) bool {
		st = *s
		if h := st.Height; h != 0 && h < height {
			return false
		}
		notFound = st.Height != height
		notFoundFull = notFound || st.StateHash == nil
		return notFoundFull
	})
	if err != nil {
		return entities.NodeStatement{},
			errors.Wrapf(err, "failed to get node statement at height %d for node '%s'", height, node)
	}
	if notFound {
		return entities.NodeStatement{},
			errors.Wrapf(ErrNoFullStatement, "no any statement at height %d for node '%s'", height, node)
	}
	if notFoundFull {
		return entities.NodeStatement{},
			errors.Wrapf(ErrNoFullStatement, "no full statement at height %d for node '%s'", height, node)
	}
	return st, nil
}

func (s *Storage) FoundStatementAtHeight(node string, height uint64) (bool, error) {
	_, err := s.GetFullStatementAtHeight(node, height)
	if err != nil {
		if errors.Is(err, ErrNoFullStatement) {
			return false, nil
		}
		return false,
			errors.Wrapf(err, "failed to get the last state hash at height %d for node '%s'", height, node)
	}
	return true, nil
}

var ErrNoFullStatement = errors.New("no full statement")
var ErrBigHeightDifference = errors.New("The height difference between nodes is more than 10")
var ErrStorageIsNotReady = errors.New("The storage has not collected enough statements for status")

func (s *Storage) findMinCommonLatestHeight(
	nodesList map[string]bool,
	minHeight uint64,
	maxHeight uint64,
) ([]entities.NodeStatement, map[string]bool, uint64, uint64, error) {
	// looking for the min common height and the max height
	var nodesHeights []entities.NodeStatement
	for node := range nodesList {
		h, err := s.LatestHeight(node)
		if err != nil && !errors.Is(err, ErrNoFullStatement) {
			return nil, nil, 0, 0, errors.Wrapf(err, "failed to get the latest height for node '%s'", node)
		}
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

	return nodesHeights, nodesList, minHeight, maxHeight, nil
}

func (s *Storage) findMinCommonSpecificHeight(
	nodesList map[string]bool,
	minHeight uint64,
) ([]entities.NodeStatement, error) {
	// looking for the min common height and the max height
	var nodesHeights []entities.NodeStatement
	for node := range nodesList {
		found, err := s.FoundStatementAtHeight(node, minHeight)
		if err != nil {
			return nil, errors.Wrapf(err,
				"failed to find min common specific height %d for node '%s'", minHeight, node,
			)
		}
		if !found {
			nodesHeights = append(nodesHeights, entities.NodeStatement{
				Node:   node,
				Status: entities.Unreachable,
			})
		} else {
			nodesHeights = append(nodesHeights, entities.NodeStatement{
				Node:   node,
				Height: minHeight,
				Status: entities.OK,
			})
		}
	}
	return nodesHeights, nil
}

func (s *Storage) FindAllStatementsOnCommonHeight(nodes []string) ([]entities.NodeStatement, error) {
	minHeight, maxHeight := uint64(math.MaxUint64), uint64(0)

	nodesList := make(map[string]bool) // reachable or unreachable
	for _, node := range nodes {
		nodesList[node] = true
	}

	nodesHeights, nodesList, minHeight, maxHeight, err := s.findMinCommonLatestHeight(nodesList, minHeight, maxHeight)
	if err != nil {
		return nil, errors.Wrap(err, "failed to find min common latest height for nodes")
	}

	if (maxHeight - minHeight) > heightDifference {
		return nodesHeights, ErrBigHeightDifference
	}

	for i := range depthCommonHeightSearch {
		sameHeight := true
		for _, node := range nodesHeights {
			if node.Height != minHeight && nodesList[node.Node] {
				sameHeight = false
				if i > 0 {
					minHeight--
					break
				}
			}
		}
		if sameHeight {
			break
		}
		nodesHeights, err = s.findMinCommonSpecificHeight(nodesList, minHeight)
		if err != nil {
			return nil, errors.Wrap(err, "failed to find min common specific height")
		}
	}
	return s.findStatementsOnHeight(nodesList, minHeight)
}

func (s *Storage) findStatementsOnHeight(nodesList map[string]bool, height uint64) ([]entities.NodeStatement, error) {
	var statementsOnHeight []entities.NodeStatement
	for node, reachable := range nodesList {
		if !reachable {
			statementsOnHeight = append(statementsOnHeight, entities.NodeStatement{
				Node:   node,
				Status: entities.Unreachable,
			})
			continue
		}
		statement, storErr := s.GetFullStatementAtHeight(node, height)
		if storErr != nil {
			if !errors.Is(storErr, ErrNoFullStatement) {
				return nil,
					errors.Wrapf(storErr, "failed to find statement at height %d for node '%s'", height, node)
			}
			s.zap.Error("failed to find statement at height",
				zap.Uint64("height", height), zap.String("node", node), zap.Error(storErr),
			)
			statementsOnHeight = append(statementsOnHeight, entities.NodeStatement{
				Node:   node,
				Status: entities.Unreachable,
			})
			continue
		}
		if statement.Height == height {
			statementsOnHeight = append(statementsOnHeight, statement)
		} else {
			s.zap.Sugar().Errorf("wrong height in statement for node %s on min height %d, received %d",
				node, height, statement.Height,
			)
			statementsOnHeight = append(statementsOnHeight, entities.NodeStatement{
				Node:   node,
				Status: entities.Unreachable,
			})
		}
	}

	return statementsOnHeight, nil
}

func (s *Storage) StateHashAtHeight(node string, height uint64) (proto.StateHash, error) {
	st, err := s.GetFullStatementAtHeight(node, height)
	if err != nil {
		return proto.StateHash{},
			errors.Wrapf(err, "failed to get state hash for node '%s' at height %d", node, height)
	}
	return *st.StateHash, nil
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
