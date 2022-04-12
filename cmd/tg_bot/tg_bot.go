package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"

	"github.com/pkg/errors"
	"nodemon/cmd/tg_bot/internal/config"
	tgBot "nodemon/cmd/tg_bot/internal/init"
	"nodemon/pkg/messaging"
)

var (
	errorInvalidParameters = errors.New("invalid parameters for telegram bot")
)

func main() {
	err := run()
	if err != nil {
		switch err {
		case context.Canceled:
			os.Exit(130)
		default:
			os.Exit(1)
		}
	}
}

func run() error {
	var (
		nanomsgURL          string
		behavior            string
		webhookLocalAddress string // only for webhook method
		publicURL           string // only for webhook method
		botToken            string
		storagePath         string
	)
	flag.StringVar(&nanomsgURL, "nano-msg-url", "ipc:///tmp/nano-msg-nodemon-pubsub.ipc", "Nanomsg IPC URL. Default is tcp://:8000")
	flag.StringVar(&behavior, "behavior", "webhook", "Behavior is either webhook or polling")
	flag.StringVar(&webhookLocalAddress, "webhook-local-address", ":8081", "The application's webhook address is :8081 by default")
	flag.StringVar(&botToken, "bot-token", "", "Temporarily: the default token is the current token")
	flag.StringVar(&publicURL, "public-url", "", "Default is https://mainnet-go-htz-fsn1-1.wavesnodes.com/bot")
	flag.StringVar(&storagePath, "storage", "", "Path to storage")
	flag.Parse()

	if botToken == "" {
		log.Println("Invalid bot token")
		return errorInvalidParameters
	}
	if behavior == config.WebhookMethod && publicURL == "" {
		log.Println("invalid public url")
		return errorInvalidParameters
	}
	if storagePath == "" {
		log.Println("invalid storage path")
		return errorInvalidParameters
	}

	ctx, done := signal.NotifyContext(context.Background(), os.Interrupt)
	defer done()

	tgBotEnv, err := tgBot.InitTgBot(behavior, webhookLocalAddress, publicURL, botToken, storagePath)
	if err != nil {
		log.Println("failed to initialize telegram bot")
		return errors.Wrap(err, "failed to init tg bot")
	}

	go func() {
		err := messaging.StartMessagingClient(ctx, nanomsgURL, tgBotEnv)
		if err != nil {
			log.Printf("failed to start messaging service: %v", err)
			return
		}
	}()

	tgBotEnv.Start()
	<-ctx.Done()
	return nil
}