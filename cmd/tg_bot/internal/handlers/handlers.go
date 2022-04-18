package handlers

import (
	"log"

	tele "gopkg.in/telebot.v3"
	"nodemon/cmd/tg_bot/internal"
	"nodemon/cmd/tg_bot/internal/messages"
	"nodemon/pkg/entities"
)

func InitHandlers(bot *tele.Bot, environment *internal.TelegramBotEnvironment) {
	bot.Handle("/hello", func(c tele.Context) error {
		oldChatID, err := environment.ChatStorage.FindChatID(entities.TelegramPlatform)
		if err != nil {
			log.Printf("failed to insert chat id into db: %v", err)
			return c.Send("An error occurred while finding the chat id in database")
		}
		if oldChatID != nil {
			return c.Send("Hello! I remember this chat.")
		}
		chatID := entities.ChatID(c.Chat().ID)

		err = environment.ChatStorage.InsertChatID(chatID, entities.TelegramPlatform)
		if err != nil {
			log.Printf("failed to insert chat id into db: %v", err)
			return c.Send("I failed to save this chat id")
		}
		return c.Send("Hello! This new chat has been saved for alerting.")
	})

	bot.Handle("/ping", func(c tele.Context) error {
		return c.Send(messages.PongText)
	})

	bot.Handle("/start", func(c tele.Context) error {
		environment.Mute = false
		return c.Send(messages.StartText)
	})

	bot.Handle("/mute", func(c tele.Context) error {
		environment.Mute = true
		return c.Send(messages.MuteText)
	})

	bot.Handle("/help", func(c tele.Context) error {
		return c.Send(
			messages.HelpInfoText,
			&tele.SendOptions{ParseMode: tele.ModeHTML})
	})
}
