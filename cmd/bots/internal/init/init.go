package init

import (
	"log"

	"github.com/bwmarrin/discordgo"
	"github.com/pkg/errors"
	tele "gopkg.in/telebot.v3"
	"nodemon/cmd/bots/internal"
	"nodemon/cmd/bots/internal/config"
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
		return nil, errors.Wrap(err, "failed to start telegram bot")
	}

	log.Printf("telegram chat id for sending alerts is %d", chatID)

	tgBotEnv := internal.NewTelegramBotEnvironment(bot, chatID, false)
	return tgBotEnv, nil
}

func InitDiscordBot(
	botToken string,
	chatID string,
) (*internal.DiscordBotEnvironment, error) {
	bot, err := discordgo.New("Bot " + botToken)
	if err != nil {
		return nil, errors.Wrap(err, "failed to start discord bot")
	}
	log.Printf("discord chat id for sending alerts is %s", chatID)

	bot.AddHandler(func(s *discordgo.Session, m *discordgo.MessageCreate) {
		if m.Author.ID == s.State.User.ID {
			return
		}
		if m.Content == "!ping" {
			_, err = s.ChannelMessageSend(chatID, "Pong!")
			if err != nil {
				log.Println(err)
			}
		}
	})

	bot.Identify.Intents = discordgo.IntentsGuildMessages | discordgo.IntentsMessageContent
	dscBotEnv := internal.NewDiscordBotEnvironment(bot, chatID)
	return dscBotEnv, nil
}
