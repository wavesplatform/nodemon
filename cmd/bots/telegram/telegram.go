package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"nodemon/cmd/bots/internal/bots"
	"nodemon/cmd/bots/internal/bots/api"
	"nodemon/cmd/bots/internal/bots/initial"
	"nodemon/cmd/bots/internal/bots/messaging"
	"nodemon/cmd/bots/internal/telegram/config"
	"nodemon/cmd/bots/internal/telegram/handlers"
	"nodemon/internal"
	generalMessaging "nodemon/pkg/messaging"
	"nodemon/pkg/messaging/pair"
	"nodemon/pkg/tools"
	"nodemon/pkg/tools/logging"
	"nodemon/pkg/tools/logging/attrs"

	"codnect.io/chrono"
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
	natsMessagingURL    string
	behavior            string
	webhookLocalAddress string // only for webhook method
	publicURL           string // only for webhook method
	tgBotToken          string
	tgChatID            int64
	development         bool
	bindAddress         string
	scheme              string
	logLevel            string
	logType             string
}

func newTelegramBotConfig() *telegramBotConfig {
	c := new(telegramBotConfig)
	tools.StringVarFlagWithEnv(&c.natsMessagingURL, "nats-msg-url",
		"nats://127.0.0.1:4222", "NATS server URL for messaging")
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
		"Logging level. Supported levels: DEBUG, INFO, WARN, ERROR.")
	tools.StringVarFlagWithEnv(&c.logType, "log-type", "pretty",
		"Set the logger output format. Supported types: text, json, pretty.")
	tools.BoolVarFlagWithEnv(&c.development, "development", false, "Development mode.")
	tools.StringVarFlagWithEnv(&c.bindAddress, "bind", "",
		"Local network address to bind the HTTP API of the service on.")
	tools.StringVarFlagWithEnv(&c.scheme, "scheme",
		"", "Blockchain scheme i.e. mainnet, testnet, stagenet")
	return c
}

func (c *telegramBotConfig) validate(logger *slog.Logger) error {
	if c.tgBotToken == "" {
		logger.Error("Telegram bot token is required")
		return bots.ErrInvalidParameters
	}
	if c.behavior == config.WebhookMethod && c.publicURL == "" {
		logger.Error("Public url is required for webhook method")
		return bots.ErrInvalidParameters
	}
	if c.scheme == "" {
		logger.Error("The blockchain scheme must be specified")
		return bots.ErrInvalidParameters
	}
	if c.tgChatID == 0 {
		logger.Error("Telegram chat ID is required")
		return bots.ErrInvalidParameters
	}
	return nil
}

func runTelegramBot() error {
	cfg := newTelegramBotConfig()
	flag.Parse()

	logger, lErr := logging.SetupLogger(cfg.logLevel, cfg.logType)
	if lErr != nil {
		return fmt.Errorf("failed to setup logger with level %q and type %q: %w", cfg.logLevel, cfg.logType, lErr)
	}

	logger.Info("Starting telegram bot", slog.String("version", internal.Version()))

	if validationErr := cfg.validate(logger); validationErr != nil {
		return validationErr
	}

	ctx, done := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer done()

	requestChan := make(chan pair.Request)
	responseChan := make(chan pair.Response)

	tgBotEnv, initErr := initial.InitTgBot(cfg.behavior, cfg.webhookLocalAddress, cfg.publicURL,
		cfg.tgBotToken, cfg.tgChatID, logger, requestChan, responseChan, cfg.scheme)
	if initErr != nil {
		logger.Error("Failed to initialize telegram bot", attrs.Error(initErr))
		return initErr
	}

	handlers.InitTgHandlers(tgBotEnv, logger, requestChan, responseChan)

	runMessagingClients(ctx, cfg, tgBotEnv, logger, requestChan, responseChan)

	if cfg.bindAddress != "" {
		botAPI, apiErr := api.NewBotAPI(cfg.bindAddress, requestChan, responseChan, defaultAPIReadTimeout,
			logger, cfg.development,
		)
		if apiErr != nil {
			logger.Error("Failed to initialize bot API", attrs.Error(apiErr))
			return apiErr
		}
		if startErr := botAPI.StartCtx(ctx); startErr != nil {
			logger.Error("Failed to start API", attrs.Error(startErr))
			return startErr
		}
		defer botAPI.Shutdown()
	}

	taskScheduler := chrono.NewDefaultTaskScheduler()
	err := bots.ScheduleNodesStatus(taskScheduler, requestChan, responseChan, tgBotEnv, logger)
	if err != nil {
		taskScheduler.Shutdown()
		logger.Error("Failed to schedule nodes status alert", attrs.Error(err))
		return err
	}
	logger.Info("Nodes status alert has been scheduled successfully")

	err = tgBotEnv.Start(ctx)
	if err != nil {
		logger.Error("Failed to start telegram bot", attrs.Error(err))
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
	tgBotEnv *bots.TelegramBotEnvironment,
	logger *slog.Logger,
	pairRequest <-chan pair.Request,
	pairResponse chan<- pair.Response,
) {
	go func() {
		err := messaging.StartSubMessagingClient(ctx, cfg.natsMessagingURL, tgBotEnv, logger)
		if err != nil {
			logger.Error("Failed to start sub messaging service", attrs.Error(err))
			panic(err)
		}
	}()

	go func() {
		topic := generalMessaging.TelegramBotRequestsTopic(cfg.scheme)
		err := messaging.StartPairMessagingClient(ctx, cfg.natsMessagingURL, pairRequest, pairResponse, logger, topic)
		if err != nil {
			logger.Error("Failed to start pair messaging service", attrs.Error(err))
			panic(err)
		}
	}()
}
