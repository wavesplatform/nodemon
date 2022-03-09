package chats

import (
	"github.com/jameycribbs/hare"
	"github.com/jameycribbs/hare/datastores/disk"
	"github.com/pkg/errors"
	"log"
	"nodemon/pkg/entities"
	"nodemon/pkg/storing/common"
)

const (
	chatsTableName = "chats"
)

type chat struct {
	ID   int `json:"id"`
	Chat entities.Chat
}

func (c *chat) GetID() int {
	return c.ID
}

func (c *chat) SetID(id int) {
	c.ID = id
}

// AfterFind required by Hare function
func (c *chat) AfterFind(_ *hare.Database) error {
	return nil
}

type Storage struct {
	db *hare.Database
}

func NewStorage(path string) (*Storage, error) {
	ds, err := disk.New(path, common.DefaultStorageExtension)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to open nodes storage at '%s'", path)
	}
	db, err := hare.New(ds)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to open nodes storage at '%s'", path)
	}
	if !db.TableExists(chatsTableName) {
		if err := db.CreateTable(chatsTableName); err != nil {
			return nil, errors.Wrapf(err, "failed to initialize chat storage at '%s'", path)
		}
	}
	cs := &Storage{db: db}

	return cs, nil
}

func (cs *Storage) InsertChatID(chatID entities.ChatID, platform entities.Platform) error {
	id, err := cs.db.Insert(chatsTableName, &chat{Chat: entities.Chat{ChatID: chatID, Platform: platform}})
	if err != nil {
		return err
	}
	log.Printf("New chat #%d stored", id)
	return nil
}

func (cs *Storage) FindChatID(platform entities.Platform) (*entities.ChatID, error) {
	ids, err := cs.db.IDs(chatsTableName)
	if err != nil {
		return nil, err
	}
	var chats []chat
	for _, id := range ids {
		var ch chat
		if err := cs.db.Find(chatsTableName, id, &ch); err != nil {
			return nil, err
		}
		chats = append(chats, ch)
	}
	for _, chat := range chats {
		if chat.Chat.Platform == platform {
			return &chat.Chat.ChatID, nil
		}
	}
	return nil, nil
}
