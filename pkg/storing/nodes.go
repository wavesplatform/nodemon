package storing

import "nodemon/pkg/entities"

type NodesStorage interface {
	Nodes() ([]entities.Node, error)
	EnabledNodes() ([]entities.Node, error)
	InsertIfNew(url string) error
	Close() error
}
