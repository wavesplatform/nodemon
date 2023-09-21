package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"nodemon/cmd/bots/internal/common"
	"nodemon/cmd/bots/internal/common/api"
	"nodemon/cmd/bots/internal/common/initial"
	"nodemon/cmd/bots/internal/common/messaging"
	"nodemon/cmd/bots/internal/telegram/config"
	"nodemon/cmd/bots/internal/telegram/handlers"
	"nodemon/pkg/messaging/pair"
	"nodemon/pkg/tools"

	"github.com/procyon-projects/chrono"
	"go.uber.org/zap"
)

const defaultAPIReadTimeout = 30 * time.Second

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
	development         bool
	bindAddress         string
}

func newTelegramBotConfig() *telegramBotConfig {
	c := new(telegramBotConfig)
	tools.StringVarFlagWithEnv(&c.nanomsgPubSubURL, "nano-msg-pubsub-url",
		"ipc:///tmp/telegram/nano-msg-nodemon-pubsub.ipc", "Nanomsg IPC URL for pubsub socket")
	tools.StringVarFlagWithEnv(&c.nanomsgPairURL, "nano-msg-pair-telegram-url",
		"ipc:///tmp/nano-msg-nodemon-pair.ipc", "Nanomsg IPC URL for pair socket")
	tools.StringVarFlagWithEnv(&c.behavior, "behavior", "webhook",
		"Behavior is either webhook or polling")
	tools.StringVarFlagWithEnv(&c.webhookLocalAddress, "webhook-local-address",
		":8081", "The application's webhook address is :8081 by default")
	tools.StringVarFlagWithEnv(&c.tgBotToken, "tg-bot-token", "",
		"The secret token used to authenticate the bot")
	tools.StringVarFlagWithEnv(&c.publicURL, "public-url", "",
		"The public url for webhook only")
	tools.Int64VarFlagWithEnv(&c.tgChatID, "telegram-chat-id",
		0, "telegram chat ID to send alerts through")
	tools.StringVarFlagWithEnv(&c.logLevel, "log-level", "INFO",
		"Logging level. Supported levels: DEBUG, INFO, WARN, ERROR, FATAL. Default logging level INFO.")
	tools.BoolVarFlagWithEnv(&c.development, "development", false, "Development mode.")
	tools.StringVarFlagWithEnv(&c.bindAddress, "bind", "",
		"Local network address to bind the HTTP API of the service on.")
	return c
}

func (c *telegramBotConfig) validate(logger *zap.Logger) error {
	if c.tgBotToken == "" {
		logger.Error("telegram bot token is required")
		return common.ErrInvalidParameters
	}
	if c.behavior == config.WebhookMethod && c.publicURL == "" {
		logger.Error("public url is required for webhook method")
		return common.ErrInvalidParameters
	}
	if c.tgChatID == 0 {
		logger.Error("telegram chat ID is required")
		return common.ErrInvalidParameters
	}
	return nil
}

func runTelegramBot() error {
	cfg := newTelegramBotConfig()
	flag.Parse()

	logger, atom, err := tools.SetupZapLogger(cfg.logLevel, cfg.development)
	if err != nil {
		log.Printf("Failed to setup zap logger: %v", err)
		return common.ErrInvalidParameters
	}

	defer func(zap *zap.Logger) {
		if syncErr := zap.Sync(); syncErr != nil {
			log.Println(syncErr)
		}
	}(logger)

	if validationErr := cfg.validate(logger); validationErr != nil {
		return validationErr
	}

	ctx, done := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer done()

	requestChan := make(chan pair.Request)
	responseChan := make(chan pair.Response)

	tgBotEnv, initErr := initial.InitTgBot(cfg.behavior, cfg.webhookLocalAddress, cfg.publicURL,
		cfg.tgBotToken, cfg.tgChatID, logger, requestChan, responseChan,
	)
	if initErr != nil {
		logger.Fatal("failed to initialize telegram bot", zap.Error(initErr))
	}

	handlers.InitTgHandlers(tgBotEnv, logger, requestChan, responseChan)

	runMessagingClients(ctx, cfg, tgBotEnv, logger, requestChan, responseChan)

	if cfg.bindAddress != "" {
		botAPI, apiErr := api.NewBotAPI(cfg.bindAddress, requestChan, responseChan, defaultAPIReadTimeout,
			logger, atom, cfg.development,
		)
		if apiErr != nil {
			logger.Error("Failed to initialize bot API", zap.Error(apiErr))
			return apiErr
		}
		if startErr := botAPI.Start(); startErr != nil {
			logger.Error("Failed to start API", zap.Error(startErr))
			return startErr
		}
		defer botAPI.Shutdown()
	}

	taskScheduler := chrono.NewDefaultTaskScheduler()
	err = common.ScheduleNodesStatus(taskScheduler, requestChan, responseChan, tgBotEnv, logger)
	if err != nil {
		taskScheduler.Shutdown()
		logger.Fatal("failed to schedule nodes status alert", zap.Error(err))
	}
	logger.Info("Nodes status alert has been scheduled successfully")

	err = tgBotEnv.Start(ctx)
	if err != nil {
		logger.Fatal("failed to start telegram bot", zap.Error(err))
		return err
	}
	<-ctx.Done()

	if !taskScheduler.IsShutdown() {
		<-taskScheduler.Shutdown()
		logger.Info("Task scheduler has been shutdown successfully")
	}
	logger.Info("Telegram bot finished")
	return nil
}

func runMessagingClients(
	ctx context.Context,
	cfg *telegramBotConfig,
	tgBotEnv *common.TelegramBotEnvironment,
	logger *zap.Logger,
	pairRequest <-chan pair.Request,
	pairResponse chan<- pair.Response,
) {
	go func() {
		err := messaging.StartSubMessagingClient(ctx, cfg.nanomsgPubSubURL, tgBotEnv, logger)
		if err != nil {
			logger.Fatal("failed to start sub messaging service", zap.Error(err))
		}
	}()

	go func() {
		err := messaging.StartPairMessagingClient(ctx, cfg.nanomsgPairURL, pairRequest, pairResponse, logger)
		if err != nil {
			logger.Fatal("failed to start pair messaging service", zap.Error(err))
		}
	}()
}
