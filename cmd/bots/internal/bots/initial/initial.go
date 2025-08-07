package initial

import (
	"log/slog"

	"nodemon/cmd/bots/internal/bots"
	"nodemon/cmd/bots/internal/telegram/config"
	"nodemon/pkg/messaging/pair"

	"github.com/bwmarrin/discordgo"
	"github.com/pkg/errors"
	"gopkg.in/telebot.v3"
)

func InitTgBot(behavior string,
	webhookLocalAddress string,
	publicURL string,
	botToken string,
	chatID int64,
	logger *slog.Logger,
	requestType chan<- pair.Request,
	responsePairType <-chan pair.Response,
	scheme string,
) (*bots.TelegramBotEnvironment, error) {
	botSettings, err := config.NewTgBotSettings(behavior, webhookLocalAddress, publicURL, botToken)
	if err != nil {
		return nil, errors.Wrap(err, "failed to set up bot configuration")
	}
	bot, err := telebot.NewBot(*botSettings)
	if err != nil {
		return nil, errors.Wrap(err, "failed to start telegram bot")
	}

	logger.Debug("Telegram chat id for sending alerts is", slog.Int64("chatID", chatID))

	tgBotEnv := bots.NewTelegramBotEnvironment(bot, chatID, false, logger, requestType, responsePairType, scheme)
	return tgBotEnv, nil
}

func InitDiscordBot(
	botToken string,
	chatID string,
	logger *slog.Logger,
	requestType chan<- pair.Request,
	responsePairType <-chan pair.Response,
	scheme string,
) (*bots.DiscordBotEnvironment, error) {
	bot, err := discordgo.New("Bot " + botToken)
	if err != nil {
		return nil, errors.Wrap(err, "failed to start discord bot")
	}
	logger.Debug("Discord chat id for sending alerts is", slog.String("chatID", chatID))

	bot.Identify.Intents = discordgo.IntentsGuildMessages | discordgo.IntentsMessageContent
	dscBotEnv := bots.NewDiscordBotEnvironment(bot, chatID, logger, requestType, responsePairType, scheme)
	return dscBotEnv, nil
}
