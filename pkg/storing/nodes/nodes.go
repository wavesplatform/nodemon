package nodes

import (
	"strings"

	"github.com/jameycribbs/hare"
	"github.com/jameycribbs/hare/datastores/disk"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"nodemon/pkg/entities"
	"nodemon/pkg/storing/common"
)

const (
	nodesTableName         = "nodes"
	specificNodesTableName = "specific_nodes"
)

type nodeRecord struct {
	ID int `json:"id"`
	entities.Node
}

func (n *nodeRecord) GetID() int {
	return n.ID
}

func (n *nodeRecord) SetID(id int) {
	n.ID = id
}

// AfterFind required by Hare function. Don't ask why.
func (n *nodeRecord) AfterFind(_ *hare.Database) error {
	return nil
}

type Storage struct {
	db  *hare.Database
	zap *zap.Logger
}

func NewStorage(path string, nodes string, logger *zap.Logger) (*Storage, error) {
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
	if !db.TableExists(specificNodesTableName) {
		if err := db.CreateTable(specificNodesTableName); err != nil {
			return nil, errors.Wrapf(err, "failed to initialize specific nodes storage at '%s'", path)
		}
	}
	cs := &Storage{db: db, zap: logger}
	err = cs.populate(nodes)
	if err != nil {
		return nil, err
	}
	return cs, nil
}

func (cs *Storage) Close() error {
	return cs.db.Close()
}

func (cs *Storage) Nodes(specific bool) ([]entities.Node, error) {
	nodesRecord, err := cs.queryNodes(func(_ nodeRecord) bool { return true }, 0, specific)
	if err != nil {
		return nil, err
	}
	return nodesFromRecords(nodesRecord), nil
}

func (cs *Storage) EnabledNodes() ([]entities.Node, error) {
	nodesRecords, err := cs.queryNodes(func(n nodeRecord) bool { return n.Enabled }, 0, false)
	if err != nil {
		return nil, err
	}
	return nodesFromRecords(nodesRecords), nil
}

func (cs *Storage) EnabledSpecificNodes() ([]entities.Node, error) {
	nodesRecords, err := cs.queryNodes(func(n nodeRecord) bool { return n.Enabled }, 0, true)
	if err != nil {
		return nil, err
	}
	return nodesFromRecords(nodesRecords), nil
}

// Update handles both specific and non-specific nodes
func (cs *Storage) Update(nodeToUpdate entities.Node) error {
	specific := false
	ids, err := cs.queryNodes(func(n nodeRecord) bool { return n.URL == nodeToUpdate.URL }, 0, false)
	if err != nil {
		return err
	}
	if len(ids) == 0 {
		// look for the url in the specific nodes table
		ids, err = cs.queryNodes(func(n nodeRecord) bool { return n.URL == nodeToUpdate.URL }, 0, true)
		if err != nil {
			return err
		}
		specific = true
		if len(ids) == 0 {
			return errors.Errorf("nodeRecord %s was not found in the storage", nodeToUpdate.URL)
		}
	}
	if len(ids) != 1 {
		return errors.Errorf("failed to update a nodeRecord in the storage, multiple nodes were found")
	}
	tableName := nodesTableName
	if specific {
		tableName = specificNodesTableName
	}

	pulledRecord := ids[0]
	err = cs.db.Update(tableName, &nodeRecord{Node: entities.Node{
		URL:     pulledRecord.URL,
		Enabled: pulledRecord.Enabled,
		Alias:   nodeToUpdate.Alias,
	}, ID: pulledRecord.ID})
	if err != nil {
		return err
	}
	cs.zap.Sugar().Infof("New nodeRecord '%s' was updated with alias %s", pulledRecord.URL, nodeToUpdate.Alias)
	return nil
}

func (cs *Storage) InsertIfNew(url string, specific bool) error {
	ids, err := cs.queryNodes(func(n nodeRecord) bool { return n.URL == url }, 0, false)
	if err != nil {
		return err
	}
	tableName := nodesTableName
	if specific {
		tableName = specificNodesTableName
	}
	if len(ids) == 0 {
		id, err := cs.db.Insert(tableName, &nodeRecord{Node: entities.Node{
			URL:     url,
			Enabled: true,
		}})
		if err != nil {
			return err
		}
		cs.zap.Sugar().Infof("New nodeRecord #%d at '%s' was stored", id, url)
	}
	return nil
}

func (cs *Storage) Delete(url string) error {
	ids, err := cs.db.IDs(nodesTableName)
	if err != nil {
		return err
	}
	for _, id := range ids {
		var n nodeRecord
		if err := cs.db.Find(nodesTableName, id, &n); err != nil {
			return err
		}
		if n.URL == url {
			err := cs.db.Delete(nodesTableName, id)
			if err != nil {
				return err
			}
			cs.zap.Sugar().Infof("Node #%d at '%s' was deleted", id, url)

		}
	}

	return nil
}

func (cs *Storage) FindAlias(url string) (string, error) {
	ids, err := cs.queryNodes(func(n nodeRecord) bool { return n.URL == url }, 0, false)
	if err != nil {
		return "", err
	}
	if len(ids) == 0 {
		// look for the url in the specific nodes table
		ids, err = cs.queryNodes(func(n nodeRecord) bool { return n.URL == url }, 0, true)
		if err != nil {
			return "", err
		}
		if len(ids) == 0 {
			return "", errors.Errorf("nodeRecord %s was not found in the storage", url)
		}
	}
	if len(ids) != 1 {
		return "", errors.Errorf("failed to update a nodeRecord in the storage, multiple nodes were found")
	}

	return ids[0].Alias, nil
}

func (cs *Storage) queryNodes(queryFn func(n nodeRecord) bool, limit int, specific bool) ([]nodeRecord, error) {
	table := nodesTableName
	if specific {
		table = specificNodesTableName
	}
	var results []nodeRecord
	ids, err := cs.db.IDs(table)
	if err != nil {
		return nil, err
	}
	for _, id := range ids {
		var n nodeRecord
		if err := cs.db.Find(table, id, &n); err != nil {
			return nil, err
		}
		if queryFn(n) {
			n.ID = id
			results = append(results, n)
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
		err = cs.InsertIfNew(url, false)
		if err != nil {
			return err
		}
	}
	return nil
}

func nodesFromRecords(records []nodeRecord) []entities.Node {
	var nodes []entities.Node
	for _, r := range records {
		nodes = append(nodes, r.Node)
	}
	return nodes
}
