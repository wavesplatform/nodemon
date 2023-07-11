package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"os"
	"os/signal"

	"nodemon/cmd/bots/internal/common"
	"nodemon/cmd/bots/internal/common/initial"
	"nodemon/cmd/bots/internal/common/messaging/pair"
	"nodemon/cmd/bots/internal/common/messaging/pubsub"
	"nodemon/cmd/bots/internal/telegram/config"
	"nodemon/cmd/bots/internal/telegram/handlers"
	pairResponses "nodemon/pkg/messaging/pair"

	"github.com/procyon-projects/chrono"
	gow "github.com/wavesplatform/gowaves/pkg/util/common"
	zapLogger "go.uber.org/zap"
)

func main() {
	const contextCanceledExitCode = 130

	if err := runTelegramBot(); err != nil {
		switch {
		case errors.Is(err, context.Canceled):
			os.Exit(contextCanceledExitCode)
		default:
			log.Fatal(err)
		}
	}
}

type telegramBotConfig struct {
	nanomsgPubSubURL    string
	nanomsgPairURL      string
	behavior            string
	webhookLocalAddress string // only for webhook method
	publicURL           string // only for webhook method
	tgBotToken          string
	tgChatID            int64
	logLevel            string
}

func newTelegramBotConfig() *telegramBotConfig {
	c := new(telegramBotConfig)
	flag.StringVar(&c.nanomsgPubSubURL, "nano-msg-pubsub-url",
		"ipc:///tmp/telegram/nano-msg-nodemon-pubsub.ipc", "Nanomsg IPC URL for pubsub socket")
	flag.StringVar(&c.nanomsgPairURL, "nano-msg-pair-telegram-url",
		"ipc:///tmp/nano-msg-nodemon-pair.ipc", "Nanomsg IPC URL for pair socket")
	flag.StringVar(&c.behavior, "behavior", "webhook",
		"Behavior is either webhook or polling")
	flag.StringVar(&c.webhookLocalAddress, "webhook-local-address",
		":8081", "The application's webhook address is :8081 by default")
	flag.StringVar(&c.tgBotToken, "tg-bot-token", "",
		"The secret token used to authenticate the bot")
	flag.StringVar(&c.publicURL, "public-url", "",
		"The public url for websocket only")
	flag.Int64Var(&c.tgChatID, "telegram-chat-id",
		0, "telegram chat ID to send alerts through")
	flag.StringVar(&c.logLevel, "log-level", "INFO",
		"Logging level. Supported levels: DEBUG, INFO, WARN, ERROR, FATAL. Default logging level INFO.")
	return c
}

func (c *telegramBotConfig) validate(zap *zapLogger.Logger) error {
	if c.tgBotToken == "" {
		zap.Error("telegram bot token is required")
		return common.ErrInvalidParameters
	}
	if c.behavior == config.WebhookMethod && c.publicURL == "" {
		zap.Error("public url is required for webhook method")
		return common.ErrInvalidParameters
	}
	if c.tgChatID == 0 {
		zap.Error("telegram chat ID is required")
		return common.ErrInvalidParameters
	}
	return nil
}

func runTelegramBot() error {
	cfg := newTelegramBotConfig()
	flag.Parse()

	zap, _ := gow.SetupLogger(cfg.logLevel)

	defer func(zap *zapLogger.Logger) {
		if err := zap.Sync(); err != nil {
			log.Println(err)
		}
	}(zap)

	if err := cfg.validate(zap); err != nil {
		return err
	}

	ctx, done := signal.NotifyContext(context.Background(), os.Interrupt)
	defer done()

	pairRequest := make(chan pairResponses.Request)
	pairResponse := make(chan pairResponses.Response)

	tgBotEnv, initErr := initial.InitTgBot(cfg.behavior, cfg.webhookLocalAddress, cfg.publicURL,
		cfg.tgBotToken, cfg.tgChatID, zap, pairRequest, pairResponse,
	)
	if initErr != nil {
		zap.Fatal("failed to initialize telegram bot", zapLogger.Error(initErr))
	}

	handlers.InitTgHandlers(tgBotEnv, zap, pairRequest, pairResponse)

	go func() {
		err := pubsub.StartSubMessagingClient(ctx, cfg.nanomsgPubSubURL, tgBotEnv, zap)
		if err != nil {
			zap.Fatal("failed to start sub messaging service", zapLogger.Error(err))
		}
	}()

	go func() {
		err := pair.StartPairMessagingClient(ctx, cfg.nanomsgPairURL, pairRequest, pairResponse, zap)
		if err != nil {
			zap.Fatal("failed to start pair messaging service", zapLogger.Error(err))
		}
	}()

	taskScheduler := chrono.NewDefaultTaskScheduler()
	err := common.ScheduleNodesStatus(taskScheduler, pairRequest, pairResponse, tgBotEnv, zap)
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
