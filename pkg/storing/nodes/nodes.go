package nodes

import (
	"log"
	"nodemon/pkg/storing/common"
	"strings"

	"github.com/jameycribbs/hare"
	"github.com/jameycribbs/hare/datastores/disk"
	"github.com/pkg/errors"
	"nodemon/pkg/entities"
)

const (
	nodesTableName = "nodes"
)

type node struct {
	ID int `json:"id"`
	entities.Node
}

func (n *node) GetID() int {
	return n.ID
}

func (n *node) SetID(id int) {
	n.ID = id
}

// AfterFind required by Hare function. Don't ask why.
func (n *node) AfterFind(_ *hare.Database) error {
	return nil
}

type Storage struct {
	db *hare.Database
}

func NewStorage(path string, nodes string) (*Storage, error) {
	ds, err := disk.New(path, common.DefaultStorageExtension)
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
	cs := &Storage{db: db}
	err = cs.populate(nodes)
	if err != nil {
		return nil, err
	}
	return cs, nil
}

func (cs *Storage) Close() error {
	return cs.db.Close()
}

func (cs *Storage) Nodes() ([]entities.Node, error) {
	return cs.queryNodes(func(_ node) bool { return true }, 0)
}

func (cs *Storage) EnabledNodes() ([]entities.Node, error) {
	return cs.queryNodes(func(n node) bool { return n.Enabled }, 0)
}

func (cs *Storage) InsertIfNew(url string) error {
	ids, err := cs.queryNodes(func(n node) bool { return n.URL == url }, 0)
	if err != nil {
		return err
	}
	if len(ids) == 0 {
		id, err := cs.db.Insert(nodesTableName, &node{Node: entities.Node{
			URL:     url,
			Enabled: true,
		}})
		if err != nil {
			return err
		}
		log.Printf("New node #%d at '%s' stored", id, url)
	}
	return nil
}

func (cs *Storage) queryNodes(queryFn func(n node) bool, limit int) ([]entities.Node, error) {
	var results []entities.Node
	ids, err := cs.db.IDs(nodesTableName)
	if err != nil {
		return nil, err
	}
	for _, id := range ids {
		var n node
		if err := cs.db.Find(nodesTableName, id, &n); err != nil {
			return nil, err
		}
		if queryFn(n) {
			results = append(results, n.Node)
		}
		if limit != 0 && limit == len(results) {
			break
		}
	}
	return results, nil
}

func (cs *Storage) populate(nodes string) error {
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
