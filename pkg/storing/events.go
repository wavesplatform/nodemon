package storing

import (
	"time"

	"nodemon/pkg/entities"
)

const (
	defaultRetentionDuration = 24 * time.Hour
	defaultPartitionDuration = 60 * time.Minute
)

type EventsStorage struct {
}

func NewEventsStorage() *EventsStorage {
	return &EventsStorage{}
}

func (s *EventsStorage) Close() error {
	return nil
}

func (s *EventsStorage) PutEvent(event entities.Event) error {
	return nil
}
