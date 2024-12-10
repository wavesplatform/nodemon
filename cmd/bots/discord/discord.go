package main

import (
	"context"
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
	"nodemon/cmd/bots/internal/discord/handlers"
	"nodemon/internal"
	"nodemon/pkg/messaging/pair"
	"nodemon/pkg/tools"

	"codnect.io/chrono"
	"github.com/pkg/errors"
	"go.uber.org/zap"
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
	natsPubSubURL   string
	natsPairURL     string
	discordBotToken string
	discordChatID   string
	logLevel        string
	development     bool
	bindAddress     string
	scheme          string
}

func newDiscordBotConfigConfig() *discordBotConfig {
	c := new(discordBotConfig)
	tools.StringVarFlagWithEnv(&c.natsPubSubURL, "nats-pubsub-url",
		"nats://127.0.0.1:4222", "NATS server URL for pubsub messaging")
	tools.StringVarFlagWithEnv(&c.natsPairURL, "nats-pair-discord-url",
		"nats://127.0.0.1:4222", "NATS server URL for pair messaging")
	tools.StringVarFlagWithEnv(&c.discordBotToken, "discord-bot-token",
		"", "The secret token used to authenticate the bot")
	tools.StringVarFlagWithEnv(&c.discordChatID, "discord-chat-id",
		"", "discord chat ID to send alerts through")
	tools.StringVarFlagWithEnv(&c.logLevel, "log-level", "INFO",
		"Logging level. Supported levels: DEBUG, INFO, WARN, ERROR, FATAL. Default logging level INFO.")
	tools.BoolVarFlagWithEnv(&c.development, "development", false, "Development mode.")
	tools.StringVarFlagWithEnv(&c.bindAddress, "bind", "",
		"Local network address to bind the HTTP API of the service on.")
	tools.StringVarFlagWithEnv(&c.scheme, "scheme",
		"testnet", "Blockchain scheme i.e. mainnet, testnet, stagenet. Used in messaging service")
	return c
}

func (c *discordBotConfig) validate(zap *zap.Logger) error {
	if c.discordBotToken == "" {
		zap.Error("discord bot token is required")
		return common.ErrInvalidParameters
	}
	if c.scheme == "" {
		zap.Error("the blockchain scheme must be specified")
	}
	if c.discordChatID == "" {
		zap.Error("discord chat ID is required")
		return common.ErrInvalidParameters
	}
	return nil
}

func runDiscordBot() error {
	cfg := newDiscordBotConfigConfig()
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

	logger.Info("Starting discord bot", zap.String("version", internal.Version()))

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
	err = common.ScheduleNodesStatus(taskScheduler, requestChan, responseChan, discordBotEnv, logger)
	if err != nil {
		taskScheduler.Shutdown()
		logger.Fatal("failed to schedule nodes status", zap.Error(err))
		return err
	}

	logger.Info("Nodes status has been scheduled successfully")

	err = discordBotEnv.Start()
	if err != nil {
		logger.Fatal("failed to start discord bot", zap.Error(err))
		return err
	}
	defer func() {
		closeErr := discordBotEnv.Bot.Close()
		if err != nil {
			logger.Error("failed to close discord bot web socket", zap.Error(closeErr))
		}
	}()
	<-ctx.Done()

	waitScheduler(taskScheduler, logger)
	logger.Info("Discord bot finished")
	return nil
}

func waitScheduler(taskScheduler chrono.TaskScheduler, logger *zap.Logger) {
	if !taskScheduler.IsShutdown() {
		<-taskScheduler.Shutdown()
		logger.Info("Task scheduler has been shutdown successfully")
	}
}

func runMessagingClients(
	ctx context.Context,
	cfg *discordBotConfig,
	discordBotEnv *common.DiscordBotEnvironment,
	logger *zap.Logger,
	requestChan chan pair.Request,
	responseChan chan pair.Response,
) {
	go func() {
		clientErr := messaging.StartSubMessagingClient(ctx, cfg.natsPubSubURL, discordBotEnv, logger, cfg.scheme)
		if clientErr != nil {
			logger.Fatal("failed to start sub messaging client", zap.Error(clientErr))
			return
		}
	}()

	go func() {
		err := messaging.StartPairMessagingClient(ctx, cfg.natsPairURL, requestChan, responseChan, logger, cfg.scheme)
		if err != nil {
			logger.Fatal("failed to start pair messaging client", zap.Error(err))
			return
		}
	}()
}
