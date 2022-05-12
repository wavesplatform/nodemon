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
		if environment.Mute {
			return c.Send(messages.PongText + " I am currently sleeping" + messages.SleepingMsg)
		}
		return c.Send(messages.PongText + " I am monitoring" + messages.MonitoringMsg)
	})

	bot.Handle("/start", func(c tele.Context) error {
		if !environment.IsEligibleForAction(c.Chat().ID) {
			return c.Send("Sorry, you have no right to start me")
		}
		if environment.Mute {
			environment.Mute = false
			return c.Send("I had been asleep, but started monitoring now... " + messages.MonitoringMsg)
		}
		return c.Send("I had already been monitoring" + messages.MonitoringMsg)
	})

	bot.Handle("/mute", func(c tele.Context) error {
		if !environment.IsEligibleForAction(c.Chat().ID) {
			return c.Send("Sorry, you have no right to mute me")
		}
		if environment.Mute {
			return c.Send("I had already been sleeping, continue sleeping.." + messages.SleepingMsg)
		}
		environment.Mute = true
		return c.Send("I had been monitoring, but going to sleep now.." + messages.SleepingMsg)
	})

	bot.Handle("/help", func(c tele.Context) error {
		return c.Send(
			messages.HelpInfoText,
			&tele.SendOptions{ParseMode: tele.ModeHTML})
	})
}
