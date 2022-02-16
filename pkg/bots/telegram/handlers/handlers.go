package handlers

import (
	"fmt"
	tele "gopkg.in/telebot.v3"
	"nodemon/pkg/bots/telegram/messaging"
)

func InitHandlers(bot *tele.Bot, environment *messaging.MessageEnvironment) {


	bot.Handle("/hello", func(c tele.Context) error {
		return c.Send("Hello!")
	})
	bot.Handle("/start", func(c tele.Context) error {
		fmt.Print("I'm here on start")
		environment.Chat = c.Chat()
		environment.ReceivedChat = true
		return c.Send("Started working...")
	})

	bot.Handle("/mute", func(c tele.Context) error {
		environment.ReceivedChat = false
		environment.Chat = c.Chat()

		return c.Send("Hello!")
	})
}
