package handlers

import (
	"fmt"

	tele "gopkg.in/telebot.v3"
	"nodemon/cmd/tg_bot/internal"
	"nodemon/cmd/tg_bot/internal/messages"
)

func InitHandlers(bot *tele.Bot, environment *internal.TelegramBotEnvironment) {
	bot.Handle("/chat", func(c tele.Context) error {

		return c.Send(fmt.Sprintf("I am sending alerts through %d chat id", environment.ChatID))
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
