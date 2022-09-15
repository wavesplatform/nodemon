package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"

	"github.com/pkg/errors"
	"github.com/procyon-projects/chrono"
	"nodemon/cmd/bots/internal/common"
	initial "nodemon/cmd/bots/internal/common/init"
	"nodemon/cmd/bots/internal/telegram/config"
	"nodemon/cmd/bots/internal/telegram/handlers"
	"nodemon/pkg/messaging/pair"
	"nodemon/pkg/messaging/pubsub"
)

func main() {
	err := runTelegramBot()
	if err != nil {
		switch err {
		case context.Canceled:
			os.Exit(130)
		default:
			os.Exit(1)
		}
	}
}

func runTelegramBot() error {
	var (
		nanomsgPubSubURL    string
		nanomsgPairUrl      string
		behavior            string
		webhookLocalAddress string // only for webhook method
		publicURL           string // only for webhook method
		tgBotToken          string
		tgChatID            int64
	)
	flag.StringVar(&nanomsgPubSubURL, "nano-msg-pubsub-url", "ipc:///tmp/telegram/nano-msg-nodemon-pubsub.ipc", "Nanomsg IPC URL for pubsub socket")
	flag.StringVar(&nanomsgPairUrl, "nano-msg-pair-url", "ipc:///tmp/nano-msg-nodemon-pair.ipc", "Nanomsg IPC URL for pair socket")
	flag.StringVar(&behavior, "behavior", "webhook", "Behavior is either webhook or polling")
	flag.StringVar(&webhookLocalAddress, "webhook-local-address", ":8081", "The application's webhook address is :8081 by default")
	flag.StringVar(&tgBotToken, "tg-bot-token", "", "")
	flag.StringVar(&publicURL, "public-url", "", "The public url for websocket only")
	flag.Int64Var(&tgChatID, "telegram-chat-id", 0, "telegram chat ID to send alerts through")
	flag.Parse()

	if tgBotToken == "" {
		log.Println("telegram token is invalid")
		return common.ErrorInvalidParameters
	}
	if behavior == config.WebhookMethod && publicURL == "" {
		log.Println("invalid public url for webhook")
		return common.ErrorInvalidParameters
	}
	if tgChatID == 0 {
		log.Println("invalid telegram chat ID")
		return common.ErrorInvalidParameters
	}

	ctx, done := signal.NotifyContext(context.Background(), os.Interrupt)
	defer done()

	tgBotEnv, err := initial.InitTgBot(behavior, webhookLocalAddress, publicURL, tgBotToken, tgChatID)
	if err != nil {
		log.Println("failed to initialize telegram bot")
		return errors.Wrap(err, "failed to init tg bot")
	}

	pairRequest := make(chan pair.RequestPair)
	pairResponse := make(chan pair.ResponsePair)
	handlers.InitTgHandlers(tgBotEnv, pairRequest, pairResponse)

	go func() {
		err := pubsub.StartSubMessagingClient(ctx, nanomsgPubSubURL, tgBotEnv)
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
	common.ScheduleNodesStatus(taskScheduler, pairRequest, pairResponse, tgBotEnv)

	tgBotEnv.Start()
	<-ctx.Done()

	if !taskScheduler.IsShutdown() {
		taskScheduler.Shutdown()
		log.Println("scheduler finished")
	}
	return nil
}
