package discord

import (
	"github.com/bwmarrin/discordgo"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"nodemon/cmd/bots/internal/common"
	"nodemon/pkg/messaging/pair"
)

func InitBot(
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
