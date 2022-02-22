package handlers

import (
	tele "gopkg.in/telebot.v3"
	"nodemon/cmd/tg_bot/internal/messaging"
)

func InitHandlers(bot *tele.Bot, environment *messaging.MessageEnvironment) {
	bot.Handle("/hello", func(c tele.Context) error {
		return c.Send("Hello!")
	})

	bot.Handle("/start", func(c tele.Context) error {
		environment.Chat = c.Chat()
		environment.ReceivedChat = true
		return c.Send("Started working...")
	})

	bot.Handle("/mute", func(c tele.Context) error {
		environment.ReceivedChat = false
		environment.Chat = c.Chat()

		return c.Send("Say no more..")
	})
}
