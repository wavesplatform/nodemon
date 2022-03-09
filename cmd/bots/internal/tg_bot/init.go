package tg_bot

import (
	"github.com/pkg/errors"
	tele "gopkg.in/telebot.v3"
	"nodemon/cmd/bots/internal/tg_bot/config"
	"nodemon/cmd/bots/internal/tg_bot/handlers"
	"nodemon/pkg/messaging"
	"nodemon/pkg/storing/chats"
)

func InitTgBot(behavior string,
	webhookLocalAddress string,
	publicURL string,
	botToken string,
	storagePath string,
) (*messaging.TelegramBotEnvironment, error) {
	botSettings, err := config.NewBotSettings(behavior, webhookLocalAddress, publicURL, botToken)
	if err != nil {
		return nil, errors.Wrap(err, "failed to set up bot configuration")
	}
	bot, err := tele.NewBot(*botSettings)
	if err != nil {
		return nil, errors.Wrap(err, "failed to start bot")
	}

	stor, err := chats.NewStorage(storagePath)
	if err != nil {
		return nil, errors.Wrap(err, "failed to initialize storage")
	}

	tgBotEnv := messaging.NewTelegramBotEnvironment(bot, stor, false)
	handlers.InitHandlers(bot, tgBotEnv)
	return tgBotEnv, nil
}
