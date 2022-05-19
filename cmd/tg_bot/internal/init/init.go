package init

import (
	"log"

	"github.com/pkg/errors"
	tele "gopkg.in/telebot.v3"
	"nodemon/cmd/tg_bot/internal"
	"nodemon/cmd/tg_bot/internal/config"
)

func InitTgBot(behavior string,
	webhookLocalAddress string,
	publicURL string,
	botToken string,
	chatID int64,
) (*internal.TelegramBotEnvironment, error) {
	botSettings, err := config.NewBotSettings(behavior, webhookLocalAddress, publicURL, botToken)
	if err != nil {
		return nil, errors.Wrap(err, "failed to set up bot configuration")
	}
	bot, err := tele.NewBot(*botSettings)
	if err != nil {
		return nil, errors.Wrap(err, "failed to start bot")
	}

	log.Printf("chat id for sending alerts is %d", chatID)

	tgBotEnv := internal.NewTelegramBotEnvironment(bot, chatID, false)
	return tgBotEnv, nil
}
