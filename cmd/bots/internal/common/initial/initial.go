package initial

import (
	"net/http"

	"nodemon/cmd/bots/internal/common"
	"nodemon/cmd/bots/internal/telegram/config"
	"nodemon/pkg/messaging/pair"

	"github.com/bwmarrin/discordgo"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"gopkg.in/telebot.v3"
)

func InitTgBot(
	behavior string,
	publicURL string,
	botToken string,
	chatID int64,
	logger *zap.Logger,
	requestType chan<- pair.Request,
	responsePairType <-chan pair.Response,
	scheme string,
) (*common.TelegramBotEnvironment, http.Handler, error) {
	// optionalWebhookHandler can be nil if the behavior is not WebhookMethod
	botSettings, optionalWebhookHandler, err := config.NewTgBotSettings(behavior, publicURL, botToken, logger)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to set up bot configuration")
	}
	bot, err := telebot.NewBot(*botSettings)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to start telegram bot")
	}

	logger.Sugar().Debugf("telegram chat id for sending alerts is %d", chatID)

	tgBotEnv := common.NewTelegramBotEnvironment(
		bot,
		chatID,
		false,
		logger,
		requestType,
		responsePairType,
		scheme,
	)
	return tgBotEnv, optionalWebhookHandler, nil
}

func InitDiscordBot(
	botToken string,
	chatID string,
	logger *zap.Logger,
	requestType chan<- pair.Request,
	responsePairType <-chan pair.Response,
	scheme string,
) (*common.DiscordBotEnvironment, error) {
	bot, err := discordgo.New("Bot " + botToken)
	if err != nil {
		return nil, errors.Wrap(err, "failed to start discord bot")
	}
	logger.Sugar().Debugf("discord chat id for sending alerts is %s", chatID)

	bot.Identify.Intents = discordgo.IntentsGuildMessages | discordgo.IntentsMessageContent
	dscBotEnv := common.NewDiscordBotEnvironment(bot, chatID, logger, requestType, responsePairType, scheme)
	return dscBotEnv, nil
}
