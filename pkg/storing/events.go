package storing

import (
	"github.com/tidwall/buntdb"
	"nodemon/pkg/entities"
)

var (
	ErrNotFound = buntdb.ErrNotFound
)

type EventsStorage interface {
	GetStatement(nodeURL string, timestamp int64) (entities.NodeStatement, error)
	ViewStatementsByNodeURLWithDescendKeys(nodeURL string, iter func(*entities.NodeStatement) bool) error
	ViewStatementsByTimestamp(timestamp int64, iter func(*entities.NodeStatement) bool) error
	PutEvent(event entities.Event) error
	StatementsCount() (int, error)
	Close() error
}
