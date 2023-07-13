package initial

import (
	"nodemon/cmd/bots/internal/common"
	"nodemon/cmd/bots/internal/telegram/config"
	"nodemon/pkg/messaging/pair"

	"github.com/bwmarrin/discordgo"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"gopkg.in/telebot.v3"
)

func InitTgBot(behavior string,
	webhookLocalAddress string,
	publicURL string,
	botToken string,
	chatID int64,
	logger *zap.Logger,
	requestType chan<- pair.Request,
	responsePairType <-chan pair.Response,
) (*common.TelegramBotEnvironment, error) {
	botSettings, err := config.NewTgBotSettings(behavior, webhookLocalAddress, publicURL, botToken)
	if err != nil {
		return nil, errors.Wrap(err, "failed to set up bot configuration")
	}
	bot, err := telebot.NewBot(*botSettings)
	if err != nil {
		return nil, errors.Wrap(err, "failed to start telegram bot")
	}

	logger.Sugar().Debugf("telegram chat id for sending alerts is %d", chatID)

	tgBotEnv := common.NewTelegramBotEnvironment(bot, chatID, false, logger, requestType, responsePairType)
	return tgBotEnv, nil
}

func InitDiscordBot(
	botToken string,
	chatID string,
	logger *zap.Logger,
	requestType chan<- pair.Request,
	responsePairType <-chan pair.Response,
) (*common.DiscordBotEnvironment, error) {
	bot, err := discordgo.New("Bot " + botToken)
	if err != nil {
		return nil, errors.Wrap(err, "failed to start discord bot")
	}
	logger.Sugar().Debugf("discord chat id for sending alerts is %s", chatID)

	bot.Identify.Intents = discordgo.IntentsGuildMessages | discordgo.IntentsMessageContent
	dscBotEnv := common.NewDiscordBotEnvironment(bot, chatID, logger, requestType, responsePairType)
	return dscBotEnv, nil
}
