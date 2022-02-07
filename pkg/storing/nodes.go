package storing

import (
	"log"
	"strings"

	"github.com/jameycribbs/hare"
	"github.com/jameycribbs/hare/datastores/disk"
	"github.com/pkg/errors"
	"nodemon/pkg/entities"
)

const (
	nodesTableName          = "nodes"
	defaultStorageExtension = ".json"
)

type Node struct {
	ID      int    `json:"id"`
	URL     string `json:"url"`
	Enabled bool   `json:"enabled"`
}

func (n *Node) GetID() int {
	return n.ID
}

func (n *Node) SetID(id int) {
	n.ID = id
}

// AfterFind required by Hare function. Don't ask why.
func (n *Node) AfterFind(_ *hare.Database) error {
	*n = Node(*n)
	return nil
}

func QueryNodes(db *hare.Database, queryFn func(n Node) bool, limit int) ([]Node, error) {
	var results []Node
	ids, err := db.IDs(nodesTableName)
	if err != nil {
		return nil, err
	}
	for _, id := range ids {
		var n Node
		if err := db.Find(nodesTableName, id, &n); err != nil {
			return nil, err
		}
		if queryFn(n) {
			results = append(results, n)
		}
		if limit != 0 && limit == len(results) {
			break
		}
	}
	return results, nil
}

type NodesStorage struct {
	db *hare.Database
}

func (cs *NodesStorage) Close() error {
	return cs.db.Close()
}

func (cs *NodesStorage) Nodes() ([]Node, error) {
	return QueryNodes(cs.db, func(_ Node) bool { return true }, 0)
}

func (cs *NodesStorage) EnabledNodes() ([]Node, error) {
	return QueryNodes(cs.db, func(n Node) bool { return n.Enabled }, 0)
}

func (cs *NodesStorage) InsertIfNew(url string) error {
	ids, err := QueryNodes(cs.db, func(n Node) bool { return n.URL == url }, 0)
	if err != nil {
		return err
	}
	if len(ids) == 0 {
		id, err := cs.db.Insert(nodesTableName, &Node{URL: url, Enabled: true})
		if err != nil {
			return err
		}
		log.Printf("New node #%d at '%s' stored", id, url)
	}
	return nil
}

func (cs *NodesStorage) populate(nodes string) error {
	for _, n := range strings.Fields(nodes) {
		url, err := entities.ValidateNodeURL(n)
		if err != nil {
			return err
		}
		err = cs.InsertIfNew(url)
		if err != nil {
			return err
		}
	}
	return nil
}

func NewNodesStorage(path string, nodes string) (*NodesStorage, error) {
	ds, err := disk.New(path, defaultStorageExtension)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to open nodes storage at '%s'", path)
	}
	db, err := hare.New(ds)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to open nodes storage at '%s'", path)
	}
	if !db.TableExists(nodesTableName) {
		if err := db.CreateTable(nodesTableName); err != nil {
			return nil, errors.Wrapf(err, "failed to initialize nodes storage at '%s'", path)
		}
	}
	cs := &NodesStorage{db: db}
	err = cs.populate(nodes)
	if err != nil {
		return nil, err
	}
	return cs, nil
}
