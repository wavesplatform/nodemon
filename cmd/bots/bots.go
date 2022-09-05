package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"

	"github.com/pkg/errors"
	"github.com/procyon-projects/chrono"
	"nodemon/cmd/bots/internal/config"
	"nodemon/cmd/bots/internal/handlers"
	initial "nodemon/cmd/bots/internal/init"
	"nodemon/pkg/messaging"
	"nodemon/pkg/messaging/pair"
	"nodemon/pkg/messaging/pubsub"
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
		tgBotToken          string
		discordBotToken     string
		tgChatID            int64
		discordChatID       string
	)
	flag.StringVar(&nanomsgPubSubURL, "nano-msg-pubsub-url", "ipc:///tmp/nano-msg-nodemon-pubsub.ipc", "Nanomsg IPC URL for pubsub socket")
	flag.StringVar(&nanomsgPairUrl, "nano-msg-pair-url", "ipc:///tmp/nano-msg-nodemon-pair.ipc", "Nanomsg IPC URL for pair socket")
	flag.StringVar(&behavior, "behavior", "webhook", "Behavior is either webhook or polling")
	flag.StringVar(&webhookLocalAddress, "webhook-local-address", ":8081", "The application's webhook address is :8081 by default")
	flag.StringVar(&tgBotToken, "tg-bot-token", "", "")
	flag.StringVar(&discordBotToken, "discord-bot-token", "", "")
	flag.StringVar(&publicURL, "public-url", "", "The public url for websocket only")
	flag.Int64Var(&tgChatID, "telegram-chat-id", 0, "telegram chat ID to send alerts through")
	flag.StringVar(&discordChatID, "discord-chat-id", "", "discord chat ID to send alerts through")
	flag.Parse()

	if tgBotToken == "" || discordBotToken == "" {
		log.Println("one of the bots' tokens is invalid")
		return errorInvalidParameters
	}
	if behavior == config.WebhookMethod && publicURL == "" {
		log.Println("invalid public url for webhook")
		return errorInvalidParameters
	}
	if tgChatID == 0 {
		log.Println("invalid chat ID")
		return errorInvalidParameters
	}
	if discordChatID == "" {
		log.Println("invalid discord chat ID")
		return errorInvalidParameters
	}

	ctx, done := signal.NotifyContext(context.Background(), os.Interrupt)
	defer done()

	tgBotEnv, err := initial.InitTgBot(behavior, webhookLocalAddress, publicURL, tgBotToken, tgChatID)
	if err != nil {
		log.Println("failed to initialize telegram bot")
		return errors.Wrap(err, "failed to init tg bot")
	}

	discordBotEnv, err := initial.InitDiscordBot(discordBotToken, discordChatID)
	if err != nil {
		log.Println("failed to initialize discord bot")
		return errors.Wrap(err, "failed to init discord bot")
	}

	pairRequest := make(chan pair.RequestPair)
	pairResponse := make(chan pair.ResponsePair)
	handlers.InitTgHandlers(tgBotEnv, pairRequest, pairResponse)

	var bots []messaging.Bot
	bots = append(bots, tgBotEnv)
	bots = append(bots, discordBotEnv)
	go func() {
		err := pubsub.StartPubSubMessagingClient(ctx, nanomsgPubSubURL, bots)
		if err != nil {
			log.Printf("failed to start pubsub messaging service: %v", err)
			return
		}
	}()

	go func() {
		err := pair.StartPairMessagingClient(ctx, nanomsgPairUrl, pairRequest, pairResponse)
		if err != nil {
			log.Printf("failed to start pair messaging service: %v", err)
		}
	}()

	taskScheduler := chrono.NewDefaultTaskScheduler()
	tgBotEnv.ScheduleNodesStatus(taskScheduler, pairRequest, pairResponse)

	discordBotEnv.Start()
	tgBotEnv.Start()
	<-ctx.Done()

	discordBotEnv.Bot.Close()

	if !taskScheduler.IsShutdown() {
		taskScheduler.Shutdown()
		log.Println("scheduler finished")
	}
	return nil
}
