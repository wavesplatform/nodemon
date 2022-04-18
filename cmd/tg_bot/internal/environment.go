package internal

import (
	"fmt"
	"log"

	"gopkg.in/telebot.v3"
	"nodemon/cmd/tg_bot/internal/messages"
	"nodemon/pkg/entities"
	"nodemon/pkg/storing/chats"
)

type TelegramBotEnvironment struct {
	ChatStorage *chats.Storage
	Bot         *telebot.Bot
	Mute        bool
}

func NewTelegramBotEnvironment(bot *telebot.Bot, storage *chats.Storage, mute bool) *TelegramBotEnvironment {
	return &TelegramBotEnvironment{Bot: bot, ChatStorage: storage, Mute: mute}
}

func (tgEnv *TelegramBotEnvironment) Start() {
	log.Println("Telegram bot started")
	tgEnv.Bot.Start()
	log.Println("Telegram bot finished")
}

func (tgEnv *TelegramBotEnvironment) constructMessage(alert string, msg []byte) string {

	message := `
<b>Alert type: %s</b>
<b>Severity: Error %s</b>
<b>Details:</b>
%s
`
	return fmt.Sprintf(message, alert, messages.ErrorMsg, string(msg)+"\n")
}

func (tgEnv *TelegramBotEnvironment) SendMessage(msg []byte) {
	if tgEnv.Mute {
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

	var messageToBot string

	alertType := msg[0]
	switch alertType {
	case byte(entities.SimpleAlertType):
		messageToBot = tgEnv.constructMessage(entities.SimpleAlertNotification, msg[1:])
	case byte(entities.UnreachableAlertType):
		messageToBot = tgEnv.constructMessage(entities.UnreachableAlertNotification, msg[1:])
	case byte(entities.IncompleteAlertType):
		messageToBot = tgEnv.constructMessage(entities.IncompleteAlertNotification, msg[1:])
	case byte(entities.InvalidHeightAlertType):
		messageToBot = tgEnv.constructMessage(entities.InvalidHeightAlertNotification, msg[1:])
	case byte(entities.HeightAlertType):
		messageToBot = tgEnv.constructMessage(entities.HeightAlertNotification, msg[1:])
	case byte(entities.StateHashAlertType):
		messageToBot = tgEnv.constructMessage(entities.StateHashAlertNotification, msg[1:])
	case byte(entities.AlertFixedType):
		messageToBot = tgEnv.constructMessage(entities.AlertFixedNotification, msg[1:])
	default:
		log.Println("unknown alert type")
	}
	_, err = tgEnv.Bot.Send(
		chat,
		messageToBot,
		&telebot.SendOptions{ParseMode: telebot.ModeHTML},
	)

	if err != nil {
		log.Printf("failed to send a message to telegram, %v", err)
	}
}
