package nodes

import "nodemon/pkg/entities"

type Storage interface {
	Close() error
	Nodes(specific bool) ([]entities.Node, error)
	EnabledNodes() ([]entities.Node, error)
	EnabledSpecificNodes() ([]entities.Node, error)
	Update(nodeToUpdate entities.Node) error
	InsertIfNew(url string, specific bool) error
	Delete(url string) error
	FindAlias(url string) (string, error)
}
