package main

import (
	"context"
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
	"nodemon/cmd/bots/internal/discord/handlers"
	"nodemon/internal"
	generalMessaging "nodemon/pkg/messaging"
	"nodemon/pkg/messaging/pair"
	"nodemon/pkg/tools"
	"nodemon/pkg/tools/logging"
	"nodemon/pkg/tools/logging/attrs"

	"codnect.io/chrono"
	"github.com/pkg/errors"
)

const defaultAPIReadTimeout = 30 * time.Second

func main() {
	const contextCanceledExitCode = 130
	if err := runDiscordBot(); err != nil {
		switch {
		case errors.Is(err, context.Canceled):
			os.Exit(contextCanceledExitCode)
		default:
			log.Fatal(err)
		}
	}
}

type discordBotConfig struct {
	natsMessagingURL string
	discordBotToken  string
	discordChatID    string
	development      bool
	bindAddress      string
	scheme           string
	logLevel         string
	logType          string
}

func newDiscordBotConfigConfig() *discordBotConfig {
	c := new(discordBotConfig)
	tools.StringVarFlagWithEnv(&c.natsMessagingURL, "nats-msg-url",
		"nats://127.0.0.1:4222", "NATS server URL for messaging")
	tools.StringVarFlagWithEnv(&c.discordBotToken, "discord-bot-token",
		"", "The secret token used to authenticate the bot")
	tools.StringVarFlagWithEnv(&c.discordChatID, "discord-chat-id",
		"", "discord chat ID to send alerts through")
	tools.StringVarFlagWithEnv(&c.logLevel, "log-level", "INFO",
		"Logging level. Supported levels: DEBUG, INFO, WARN, ERROR.")
	tools.StringVarFlagWithEnv(&c.logType, "log-type", "pretty",
		"Set the logger output format. Supported types: text, json, pretty.")
	tools.BoolVarFlagWithEnv(&c.development, "development", false, "Development mode.")
	tools.StringVarFlagWithEnv(&c.bindAddress, "bind", "",
		"Local network address to bind the HTTP API of the service on.")
	tools.StringVarFlagWithEnv(&c.scheme, "scheme", "",
		"Blockchain scheme i.e. mainnet, testnet, stagenet. Used in messaging service")
	return c
}

func (c *discordBotConfig) validate(logger *slog.Logger) error {
	if c.discordBotToken == "" {
		logger.Error("Discord bot token is required")
		return bots.ErrInvalidParameters
	}
	if c.scheme == "" {
		logger.Error("The blockchain scheme must be specified")
		return bots.ErrInvalidParameters
	}
	if c.discordChatID == "" {
		logger.Error("Discord chat ID is required")
		return bots.ErrInvalidParameters
	}
	return nil
}

func runDiscordBot() error {
	cfg := newDiscordBotConfigConfig()
	flag.Parse()

	logger, lErr := logging.SetupLogger(cfg.logLevel, cfg.logType)
	if lErr != nil {
		return fmt.Errorf("failed to setup logger with level %q and type %q: %w", cfg.logLevel, cfg.logType, lErr)
	}

	logger.Info("Starting discord bot", slog.String("version", internal.Version()))

	if validationErr := cfg.validate(logger); validationErr != nil {
		return validationErr
	}

	ctx, done := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer done()

	requestChan := make(chan pair.Request)
	responseChan := make(chan pair.Response)

	discordBotEnv, initErr := initial.InitDiscordBot(
		cfg.discordBotToken,
		cfg.discordChatID,
		logger,
		requestChan,
		responseChan,
		cfg.scheme,
	)
	if initErr != nil {
		return errors.Wrap(initErr, "failed to init discord bot")
	}
	handlers.InitDscHandlers(discordBotEnv, requestChan, responseChan, logger)

	runMessagingClients(ctx, cfg, discordBotEnv, logger, requestChan, responseChan)

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
	err := bots.ScheduleNodesStatus(taskScheduler, requestChan, responseChan, discordBotEnv, logger)
	if err != nil {
		taskScheduler.Shutdown()
		logger.Error("Failed to schedule nodes status", attrs.Error(err))
		return err
	}

	logger.Info("Nodes status has been scheduled successfully")

	err = discordBotEnv.Start()
	if err != nil {
		logger.Error("Failed to start discord bot", attrs.Error(err))
		return err
	}
	defer func() {
		if closeErr := discordBotEnv.Bot.Close(); closeErr != nil {
			logger.Error("Failed to close discord bot web socket", attrs.Error(closeErr))
		}
	}()
	<-ctx.Done()

	waitScheduler(taskScheduler, logger)
	logger.Info("Discord bot finished")
	return nil
}

func waitScheduler(taskScheduler chrono.TaskScheduler, logger *slog.Logger) {
	if !taskScheduler.IsShutdown() {
		<-taskScheduler.Shutdown()
		logger.Info("Task scheduler has been shutdown successfully")
	}
}

func runMessagingClients(
	ctx context.Context,
	cfg *discordBotConfig,
	discordBotEnv *bots.DiscordBotEnvironment,
	logger *slog.Logger,
	requestChan chan pair.Request,
	responseChan chan pair.Response,
) {
	go func() {
		err := messaging.StartSubMessagingClient(ctx, cfg.natsMessagingURL, discordBotEnv, logger)
		if err != nil {
			logger.Error("Failed to start sub messaging client", attrs.Error(err))
			panic(err)
		}
	}()

	go func() {
		topic := generalMessaging.DiscordBotRequestsTopic(cfg.scheme)
		err := messaging.StartPairMessagingClient(ctx, cfg.natsMessagingURL, requestChan, responseChan, logger, topic)
		if err != nil {
			logger.Error("Failed to start pair messaging client", attrs.Error(err))
			panic(err)
		}
	}()
}
