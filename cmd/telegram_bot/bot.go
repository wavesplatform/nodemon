package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/pkg/errors"
	tele "gopkg.in/telebot.v3"
	"log"
	"nodemon/pkg/bots/telegram/config"
	"nodemon/pkg/bots/telegram/handlers"
	"nodemon/pkg/bots/telegram/messaging"
	"os"
	"os/signal"
)

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
	var (
		nanomsgURL          string
		behavior            string
		webhookLocalAddress string // only for webhook method
		publicURL           string // only for webhook method
		botToken            string
	)
	flag.StringVar(&nanomsgURL, "nano-msg-url", "tcp://:8000", "Nanomsg IPC URL. Default is tcp://:8000")
	flag.StringVar(&behavior, "behavior", "webhook", "Behavior is either webhook or polling")
	flag.StringVar(&webhookLocalAddress, "webhook-local-address", "8081", "The application's webhook address is :8081 by default")
	flag.StringVar(&botToken, "bot-token", "5054144081:AAFeGslb-lhF3ujPAA2Ogtn1U5DbcC1U36U", "Temporarily: the default token is the current token")
	flag.StringVar(&publicURL, "public-url", "https://mainnet-go-htz-fsn1-1.wavesnodes.com/bot", "Default is https://mainnet-go-htz-fsn1-1.wavesnodes.com/bot")
	flag.Parse()

	ctx, done := signal.NotifyContext(context.Background(), os.Interrupt)
	defer done()


	botConfig, err := config.NewBotConfig(behavior, webhookLocalAddress, publicURL, botToken)
	if err != nil {
		return errors.Wrap(err, "failed to set up bot configuration")
	}
	fmt.Print("no errors afteer config")
	bot, err := tele.NewBot(botConfig.Settings)
	if err != nil {
		return errors.Wrap(err, "failed to start bot")
	}

	messagingEnv := &messaging.MessageEnvironment{ReceivedChat: false}
	handlers.InitHandlers(bot, messagingEnv)

	go func() {
		err := messaging.StartMessagingClient(ctx, nanomsgURL, bot, messagingEnv)
		if err != nil {
			log.Printf("failed to start messaging service: %v", err)
			return
		}
	}()

	log.Println("started")
	bot.Start()

	log.Println("finished")
	return nil
}
