package init

import (
	"log"

	"github.com/bwmarrin/discordgo"
	"github.com/pkg/errors"
	tele "gopkg.in/telebot.v3"
	"nodemon/cmd/bots/internal/common"
	"nodemon/cmd/bots/internal/telegram/config"
)

func InitTgBot(behavior string,
	webhookLocalAddress string,
	publicURL string,
	botToken string,
	chatID int64,
) (*common.TelegramBotEnvironment, error) {
	botSettings, err := config.NewTgBotSettings(behavior, webhookLocalAddress, publicURL, botToken)
	if err != nil {
		return nil, errors.Wrap(err, "failed to set up bot configuration")
	}
	bot, err := tele.NewBot(*botSettings)
	if err != nil {
		return nil, errors.Wrap(err, "failed to start telegram bot")
	}

	log.Printf("telegram chat id for sending alerts is %d", chatID)

	tgBotEnv := common.NewTelegramBotEnvironment(bot, chatID, false)
	return tgBotEnv, nil
}

func InitDiscordBot(
	botToken string,
	chatID string,
) (*common.DiscordBotEnvironment, error) {
	bot, err := discordgo.New("Bot " + botToken)
	if err != nil {
		return nil, errors.Wrap(err, "failed to start discord bot")
	}
	log.Printf("discord chat id for sending alerts is %s", chatID)

	bot.Identify.Intents = discordgo.IntentsGuildMessages | discordgo.IntentsMessageContent
	dscBotEnv := common.NewDiscordBotEnvironment(bot, chatID)
	return dscBotEnv, nil
}
