package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"nodemon/cmd/bots/internal/common"
	"nodemon/cmd/bots/internal/common/initial"
	"nodemon/cmd/bots/internal/common/messaging"
	"nodemon/cmd/bots/internal/discord/handlers"
	"nodemon/pkg/messaging/pair"
	"nodemon/pkg/tools"

	"github.com/pkg/errors"
	"github.com/procyon-projects/chrono"
	gow "github.com/wavesplatform/gowaves/pkg/util/common"
	"go.uber.org/zap"
)

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
	nanomsgPubSubURL string
	nanomsgPairURL   string
	discordBotToken  string
	discordChatID    string
	logLevel         string
}

func newDiscordBotConfigConfig() *discordBotConfig {
	c := new(discordBotConfig)
	tools.StringVarFlagWithEnv(&c.nanomsgPubSubURL, "nano-msg-pubsub-url",
		"ipc:///tmp/discord/nano-msg-nodemon-pubsub.ipc", "Nanomsg IPC URL for pubsub socket")
	tools.StringVarFlagWithEnv(&c.nanomsgPairURL, "nano-msg-pair-discord-url",
		"ipc:///tmp/nano-msg-nodemon-pair.ipc", "Nanomsg IPC URL for pair socket")
	tools.StringVarFlagWithEnv(&c.discordBotToken, "discord-bot-token",
		"", "The secret token used to authenticate the bot")
	tools.StringVarFlagWithEnv(&c.discordChatID, "discord-chat-id",
		"", "discord chat ID to send alerts through")
	tools.StringVarFlagWithEnv(&c.logLevel, "log-level", "INFO",
		"Logging level. Supported levels: DEBUG, INFO, WARN, ERROR, FATAL. Default logging level INFO.")
	return c
}

func (c *discordBotConfig) validate(zap *zap.Logger) error {
	if c.discordBotToken == "" {
		zap.Error("discord bot token is required")
		return common.ErrInvalidParameters
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

	logger, _ := gow.SetupLogger(cfg.logLevel)

	defer func(zap *zap.Logger) {
		if err := zap.Sync(); err != nil {
			log.Println(err)
		}
	}(logger)

	if err := cfg.validate(logger); err != nil {
		return err
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
	)
	if initErr != nil {
		return errors.Wrap(initErr, "failed to init discord bot")
	}
	handlers.InitDscHandlers(discordBotEnv, requestChan, responseChan, logger)

	go func() {
		clientErr := messaging.StartSubMessagingClient(ctx, cfg.nanomsgPubSubURL, discordBotEnv, logger)
		if clientErr != nil {
			logger.Fatal("failed to start sub messaging client", zap.Error(clientErr))
			return
		}
	}()

	go func() {
		err := messaging.StartPairMessagingClient(ctx, cfg.nanomsgPairURL, requestChan, responseChan, logger)
		if err != nil {
			logger.Fatal("failed to start pair messaging client", zap.Error(err))
			return
		}
	}()

	taskScheduler := chrono.NewDefaultTaskScheduler()
	err := common.ScheduleNodesStatus(taskScheduler, requestChan, responseChan, discordBotEnv, logger)
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

	logger.Info("Discord bot finished")
	if !taskScheduler.IsShutdown() {
		taskScheduler.Shutdown()
		logger.Info("scheduler finished")
	}
	return nil
}
