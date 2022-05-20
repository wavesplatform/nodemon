package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"

	"nodemon/cmd/tg_bot/internal/handlers"
	"nodemon/pkg/messaging/pair"
	"nodemon/pkg/messaging/pubsub"

	"github.com/pkg/errors"
	"nodemon/cmd/tg_bot/internal/config"
	tgBot "nodemon/cmd/tg_bot/internal/init"
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
		nanomsgPubSubURL    string
		nanomsgPairUrl      string
		behavior            string
		webhookLocalAddress string // only for webhook method
		publicURL           string // only for webhook method
		botToken            string
		chatID              int64
	)
	flag.StringVar(&nanomsgPubSubURL, "nano-msg-pubsub-url", "ipc:///tmp/nano-msg-nodemon-pubsub.ipc", "Nanomsg IPC URL for pubsub socket")
	flag.StringVar(&nanomsgPairUrl, "nano-msg-pair-url", "ipc:///tmp/nano-msg-nodemon-pair.ipc", "Nanomsg IPC URL for pair socket")
	flag.StringVar(&behavior, "behavior", "webhook", "Behavior is either webhook or polling")
	flag.StringVar(&webhookLocalAddress, "webhook-local-address", ":8081", "The application's webhook address is :8081 by default")
	flag.StringVar(&botToken, "bot-token", "", "Temporarily: the default token is the current token")
	flag.StringVar(&publicURL, "public-url", "", "Default is https://mainnet-go-htz-fsn1-1.wavesnodes.com/bot")
	flag.Int64Var(&chatID, "chat-id", 0, "Chat ID to send alerts through")
	flag.Parse()

	if botToken == "" {
		log.Println("Invalid bot token")
		return errorInvalidParameters
	}
	if behavior == config.WebhookMethod && publicURL == "" {
		log.Println("invalid public url for webhook")
		return errorInvalidParameters
	}
	if chatID == 0 {
		log.Println("invalid chat ID")
		return errorInvalidParameters
	}

	ctx, done := signal.NotifyContext(context.Background(), os.Interrupt)
	defer done()

	tgBotEnv, err := tgBot.InitTgBot(behavior, webhookLocalAddress, publicURL, botToken, chatID)
	if err != nil {
		log.Println("failed to initialize telegram bot")
		return errors.Wrap(err, "failed to init tg bot")
	}

	pairRequest := make(chan pair.RequestPair)
	pairResponse := make(chan pair.ResponsePair)
	handlers.InitHandlers(tgBotEnv.Bot, tgBotEnv, pairRequest, pairResponse)

	go func() {
		err := pubsub.StartMessagingClient(ctx, nanomsgPubSubURL, tgBotEnv)
		if err != nil {
			log.Printf("failed to start pubsub messaging service: %v", err)
			return
		}
	}()

	go func() {
		err := pair.StartMessagingPairClient(ctx, nanomsgPairUrl, pairRequest, pairResponse)
		if err != nil {
			log.Printf("failed to start pair messaging service: %v", err)
		}
	}()

	tgBotEnv.Start()
	<-ctx.Done()
	return nil
}
