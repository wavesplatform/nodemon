package nodes

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"github.com/pkg/errors"
	"go.uber.org/zap"
	"nodemon/pkg/entities"
)

type nodes []entities.Node

func (n nodes) queryNodes(iter func(n entities.Node) bool, limit int) []entities.Node {
	var out []entities.Node
	for i, node := range n {
		if limit != 0 && i > limit {
			break
		}
		if iter(node) {
			out = append(out, node)
		}
	}
	return out
}

func (n nodes) FindNodeByURL(url string) (entities.Node, bool) {
	for _, node := range n {
		if node.URL == url {
			return node, true
		}
	}
	return entities.Node{}, false
}

func (n nodes) Update(updated entities.Node) bool {
	for i, node := range n {
		if node.URL == updated.URL {
			n[i] = updated
			return true
		}
	}
	return false
}

func appendIfNew(ns nodes, url string) (nodes, bool) {
	for _, node := range ns {
		if node.URL == url {
			return ns, false
		}
	}
	newNode := entities.Node{URL: url, Enabled: true}
	return append(ns, newNode), true
}

func deleteIfFound(ns nodes, url string) (nodes, bool) {
	for i, node := range ns {
		if node.URL == url {
			ns[i] = ns[len(ns)-1] // replace found element with the last element
			return ns[:len(ns)-1], true
		}
	}
	return ns, false
}

type dbStruct struct {
	CommonNodes   nodes `json:"common_nodes"`
	SpecificNodes nodes `json:"specific_nodes"`
}

func (db dbStruct) Nodes(specific bool) nodes {
	if specific {
		return db.SpecificNodes
	}
	return db.CommonNodes
}

func (db dbStruct) SyncToFile(path string) error {
	data, err := json.MarshalIndent(db, "", " ")
	if err != nil {
		return errors.Wrapf(err, "failed to marshal nodes db as json")
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		return errors.Wrapf(err, "failed to write nodes db data to file '%s'", path)
	}
	return nil
}

type JSONStorage struct {
	mu     *sync.RWMutex
	db     *dbStruct
	dbFile string
	zap    *zap.Logger
}

func NewJSONStorage(path string, nodes []string, logger *zap.Logger) (*JSONStorage, error) {
	path = filepath.Clean(path)
	if err := createDBFileIfNotExist(path); err != nil {
		return nil, errors.Wrapf(err, "failed to create and init nodes db file '%s'", path)
	}

	data, err := os.ReadFile(path) // now we have some guarantees that file exists
	if err != nil {
		return nil, errors.Wrap(err, "failed to read nodes file")
	}
	db := new(dbStruct)
	if err := json.Unmarshal(data, db); err != nil {
		return nil, errors.Wrapf(err, "failed to parse data as json from file '%s'", path)
	}
	s := &JSONStorage{mu: new(sync.RWMutex), db: db, dbFile: path, zap: logger}
	if err := s.populate(nodes); err != nil {
		return nil, errors.Wrapf(err, "failed to populate nodes storage")
	}
	return s, nil
}

func createDBFileIfNotExist(path string) error {
	switch _, err := os.Stat(path); {
	case err == nil:
		return nil
	case errors.Is(err, os.ErrNotExist):
		var empty dbStruct
		data, err := json.Marshal(empty)
		if err != nil {
			return errors.Wrap(err, "failed to marshal empty db struct to json")
		}
		return os.WriteFile(path, data, 0600)
	default:
		return errors.Wrap(err, "failed to get stat for file")
	}
}

func (s *JSONStorage) Close() error {
	return nil
}

func (s *JSONStorage) Nodes(specific bool) ([]entities.Node, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.db.Nodes(specific), nil
}

func (s *JSONStorage) EnabledNodes() ([]entities.Node, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.db.CommonNodes.queryNodes(func(n entities.Node) bool { return n.Enabled }, 0), nil
}

func (s *JSONStorage) EnabledSpecificNodes() ([]entities.Node, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.db.SpecificNodes.queryNodes(func(n entities.Node) bool { return n.Enabled }, 0), nil
}

func (s *JSONStorage) Update(node entities.Node) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	updated := s.db.CommonNodes.Update(node)
	if !updated {
		updated = s.db.SpecificNodes.Update(node)
	}
	if !updated {
		return nodeNotFoundErr(node.URL)
	}

	if err := s.db.SyncToFile(s.dbFile); err != nil {
		return errors.Wrapf(err, "failed to update node '%s'", node.URL)
	}
	s.zap.Sugar().Infof("Node '%s' was updated to %+v", node.URL, node)
	return nil
}

func (s *JSONStorage) InsertIfNew(url string, specific bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var appended bool
	if specific {
		s.db.SpecificNodes, appended = appendIfNew(s.db.SpecificNodes, url)
	} else {
		s.db.CommonNodes, appended = appendIfNew(s.db.CommonNodes, url)
	}
	if !appended {
		return nil
	}

	if err := s.db.SyncToFile(s.dbFile); err != nil {
		return errors.Wrapf(err, "failed to insert new node '%s' (specific=%t)", url, specific)
	}
	s.zap.Sugar().Infof("New node '%s' (specific=%t) was stored", url, specific)
	return nil
}

func (s *JSONStorage) Delete(url string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var deleted bool
	s.db.CommonNodes, deleted = deleteIfFound(s.db.CommonNodes, url)
	if !deleted {
		s.db.SpecificNodes, deleted = deleteIfFound(s.db.SpecificNodes, url)
	}
	if !deleted {
		return nodeNotFoundErr(url)
	}

	if err := s.db.SyncToFile(s.dbFile); err != nil {
		return errors.Wrapf(err, "failed to delete node '%s'", url)
	}
	s.zap.Sugar().Infof("Node '%s' was deleted", url)
	return nil
}

func (s *JSONStorage) FindAlias(url string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	n, found := s.db.CommonNodes.FindNodeByURL(url)
	if !found {
		n, found = s.db.SpecificNodes.FindNodeByURL(url)
		if !found {
			return "", nodeNotFoundErr(url)
		}
	}
	return n.Alias, nil
}

func (s *JSONStorage) populate(nodes []string) error {
	var (
		needSync  bool
		validated = make(map[string]struct{}, len(nodes))
	)
	for _, n := range nodes {
		url, err := entities.ValidateNodeURL(n)
		if err != nil {
			return errors.Wrapf(err, "failed to validate node '%s'", n)
		}
		if _, ok := validated[url]; !ok {
			validated[url] = struct{}{}
		} else {
			return errors.Errorf("found duplicate for node '%s'", n)
		}
		var appended bool
		s.db.CommonNodes, appended = appendIfNew(s.db.CommonNodes, url)
		if appended {
			needSync = true
		}
	}
	if !needSync {
		return nil
	}
	if err := s.db.SyncToFile(s.dbFile); err != nil {
		return errors.Wrap(err, "failed to populate nodes db")
	}
	return nil
}

func nodeNotFoundErr(url string) error {
	return errors.Errorf("nodeRecord '%s' was not found in the storage", url)
}
