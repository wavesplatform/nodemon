package main

import (
	"context"
	"github.com/asaskevich/govalidator"
	"github.com/joho/godotenv"
	"go.uber.org/zap"
	tb "gopkg.in/tucnak/telebot.v2"
	"nodemon/pkg/bots/telegram/config"
	"os"
)

func init() {
	err := godotenv.Load()

	if err := config.InitLog(); err != nil {
		panic(err)
	}

	if err != nil {
		zap.S().Infof("No .env file found: %v", err)
	}
	govalidator.SetFieldsRequiredByDefault(true)
}

func main() {
	err := run()
		switch err {
		case context.Canceled:
			os.Exit(130)
		case config.InvalidParameters:
			os.Exit(2)
		default:
			os.Exit(1)
		}
}

func run() error {
	appConfig, err := config.LoadAppConfig()
	if err != nil {
		return config.InvalidParameters
	}

	webhook := &tb.Webhook{
		Listen:   ":2000",
		Endpoint: &tb.WebhookEndpoint{PublicURL: appConfig.WebHookAddress()},
	}

	botSettings := tb.Settings{
		Token:  appConfig.BotToken(),
		Poller: webhook,
	}

	bot, err := tb.NewBot(botSettings)
	if err != nil {
		zap.S().Fatalf("failed to start bot, %v", err)
	}

	bot.Start()
	zap.S().Info("finished")
	return nil
}
