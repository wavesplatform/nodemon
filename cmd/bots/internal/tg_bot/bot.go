package tg_bot

import (
	"gopkg.in/telebot.v3"
	"log"
	"nodemon/pkg/entities"
	"nodemon/pkg/storing/chats"
)

type TelegramBotEnvironment struct {
	ChatStorage *chats.Storage
	Bot         *telebot.Bot
	ShutUp      bool
}

func NewTelegramBotEnvironment(bot *telebot.Bot, storage *chats.Storage, shutUp bool) *TelegramBotEnvironment {
	return &TelegramBotEnvironment{Bot: bot, ChatStorage: storage, ShutUp: shutUp}
}

func (tgEnv *TelegramBotEnvironment) Start() {
	log.Println("Telegram bot started")
	tgEnv.Bot.Start()
	log.Println("Telegram bot finished")
}

func (tgEnv *TelegramBotEnvironment) SendMessage(msg []byte) {
	if tgEnv.ShutUp {
		return
	}

	chatID, err := tgEnv.ChatStorage.FindChatID(entities.TelegramPlatform)
	if err != nil {
		log.Printf("failed to find chat id: %v", err)
		return
	}

	if chatID == nil {
		log.Println("have not received a chat id yet")
		return
	}

	chat := &telebot.Chat{ID: int64(*chatID)}
	_, err = tgEnv.Bot.Send(chat, string(msg))
	if err != nil {
		log.Printf("failed to send a message to telegram, %v", err)
	}
}
