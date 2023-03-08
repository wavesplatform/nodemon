package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"os"
	"os/signal"

	"github.com/procyon-projects/chrono"
	gow "github.com/wavesplatform/gowaves/pkg/util/common"
	zapLogger "go.uber.org/zap"
	"nodemon/cmd/bots/internal/common"
	initial "nodemon/cmd/bots/internal/common/init"
	"nodemon/cmd/bots/internal/common/messaging/pair"
	"nodemon/cmd/bots/internal/common/messaging/pubsub"
	"nodemon/cmd/bots/internal/telegram/config"
	"nodemon/cmd/bots/internal/telegram/handlers"
	pairResponses "nodemon/pkg/messaging/pair"
)

func main() {
	if err := runTelegramBot(); err != nil {
		switch {
		case errors.Is(err, context.Canceled):
			os.Exit(130)
		default:
			log.Fatal(err)
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
		logLevel            string
	)
	flag.StringVar(&nanomsgPubSubURL, "nano-msg-pubsub-url", "ipc:///tmp/telegram/nano-msg-nodemon-pubsub.ipc", "Nanomsg IPC URL for pubsub socket")
	flag.StringVar(&nanomsgPairUrl, "nano-msg-pair-telegram-url", "ipc:///tmp/nano-msg-nodemon-pair.ipc", "Nanomsg IPC URL for pair socket")
	flag.StringVar(&behavior, "behavior", "webhook", "Behavior is either webhook or polling")
	flag.StringVar(&webhookLocalAddress, "webhook-local-address", ":8081", "The application's webhook address is :8081 by default")
	flag.StringVar(&tgBotToken, "tg-bot-token", "", "The secret token used to authenticate the bot")
	flag.StringVar(&publicURL, "public-url", "", "The public url for websocket only")
	flag.Int64Var(&tgChatID, "telegram-chat-id", 0, "telegram chat ID to send alerts through")
	flag.StringVar(&logLevel, "log-level", "INFO", "Logging level. Supported levels: DEBUG, INFO, WARN, ERROR, FATAL. Default logging level INFO.")
	flag.Parse()

	zap, _ := gow.SetupLogger(logLevel)

	defer func(zap *zapLogger.Logger) {
		if err := zap.Sync(); err != nil {
			log.Println(err)
		}
	}(zap)

	if tgBotToken == "" {
		zap.Error("telegram bot token is required")
		return common.ErrorInvalidParameters
	}
	if behavior == config.WebhookMethod && publicURL == "" {
		zap.Error("public url is required for webhook method")
		return common.ErrorInvalidParameters
	}
	if tgChatID == 0 {
		zap.Error("telegram chat ID is required")
		return common.ErrorInvalidParameters
	}

	ctx, done := signal.NotifyContext(context.Background(), os.Interrupt)
	defer done()

	pairRequest := make(chan pairResponses.RequestPair)
	pairResponse := make(chan pairResponses.ResponsePair)

	tgBotEnv, err := initial.InitTgBot(behavior, webhookLocalAddress, publicURL, tgBotToken, tgChatID, zap, pairRequest, pairResponse)
	if err != nil {
		zap.Fatal("failed to initialize telegram bot", zapLogger.Error(err))
	}

	handlers.InitTgHandlers(tgBotEnv, zap, pairRequest, pairResponse)

	go func() {
		err := pubsub.StartSubMessagingClient(ctx, nanomsgPubSubURL, tgBotEnv, zap)
		if err != nil {
			zap.Fatal("failed to start sub messaging service", zapLogger.Error(err))
		}
	}()

	go func() {
		err := pair.StartPairMessagingClient(ctx, nanomsgPairUrl, pairRequest, pairResponse, zap)
		if err != nil {
			zap.Fatal("failed to start pair messaging service", zapLogger.Error(err))
		}

	}()

	taskScheduler := chrono.NewDefaultTaskScheduler()
	err = common.ScheduleNodesStatus(taskScheduler, pairRequest, pairResponse, tgBotEnv, zap)
	if err != nil {
		taskScheduler.Shutdown()
		zap.Fatal("failed to schedule nodes status alert", zapLogger.Error(err))
	}
	zap.Info("Nodes status alert has been scheduled successfully")

	err = tgBotEnv.Start()
	if err != nil {
		zap.Fatal("failed to start telegram bot", zapLogger.Error(err))
		return err
	}
	<-ctx.Done()

	if !taskScheduler.IsShutdown() {
		taskScheduler.Shutdown()
		zap.Info("Task scheduler has been shutdown successfully")
	}
	return nil
}
