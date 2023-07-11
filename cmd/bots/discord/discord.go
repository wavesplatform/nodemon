package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"

	"nodemon/cmd/bots/internal/common"
	"nodemon/cmd/bots/internal/common/initial"
	"nodemon/cmd/bots/internal/common/messaging/pair"
	"nodemon/cmd/bots/internal/common/messaging/pubsub"
	"nodemon/cmd/bots/internal/discord/handlers"
	pairCommon "nodemon/pkg/messaging/pair"

	"github.com/pkg/errors"
	"github.com/procyon-projects/chrono"
	gow "github.com/wavesplatform/gowaves/pkg/util/common"
	zapLogger "go.uber.org/zap"
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
	flag.StringVar(&c.nanomsgPubSubURL, "nano-msg-pubsub-url",
		"ipc:///tmp/discord/nano-msg-nodemon-pubsub.ipc", "Nanomsg IPC URL for pubsub socket")
	flag.StringVar(&c.nanomsgPairURL, "nano-msg-pair-discord-url",
		"ipc:///tmp/nano-msg-nodemon-pair.ipc", "Nanomsg IPC URL for pair socket")
	flag.StringVar(&c.discordBotToken, "discord-bot-token",
		"", "The secret token used to authenticate the bot")
	flag.StringVar(&c.discordChatID, "discord-chat-id",
		"", "discord chat ID to send alerts through")
	flag.StringVar(&c.logLevel, "log-level", "INFO",
		"Logging level. Supported levels: DEBUG, INFO, WARN, ERROR, FATAL. Default logging level INFO.")
	return c
}

func (c *discordBotConfig) validate(zap *zapLogger.Logger) error {
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

	requestCh := make(chan pairCommon.Request)
	responseCh := make(chan pairCommon.Response)

	discordBotEnv, initErr := initial.InitDiscordBot(cfg.discordBotToken, cfg.discordChatID, zap, requestCh, responseCh)
	if initErr != nil {
		return errors.Wrap(initErr, "failed to init discord bot")
	}
	handlers.InitDscHandlers(discordBotEnv, requestCh, responseCh, zap)

	go func() {
		clientErr := pubsub.StartSubMessagingClient(ctx, cfg.nanomsgPubSubURL, discordBotEnv, zap)
		if clientErr != nil {
			zap.Fatal("failed to start sub messaging client", zapLogger.Error(clientErr))
			return
		}
	}()

	go func() {
		err := pair.StartPairMessagingClient(ctx, cfg.nanomsgPairURL, requestCh, responseCh, zap)
		if err != nil {
			zap.Fatal("failed to start pair messaging client", zapLogger.Error(err))
			return
		}
	}()

	taskScheduler := chrono.NewDefaultTaskScheduler()
	err := common.ScheduleNodesStatus(taskScheduler, requestCh, responseCh, discordBotEnv, zap)
	if err != nil {
		taskScheduler.Shutdown()
		zap.Fatal("failed to schedule nodes status", zapLogger.Error(err))
		return err
	}

	zap.Info("Nodes status has been scheduled successfully")

	err = discordBotEnv.Start()
	if err != nil {
		zap.Fatal("failed to start discord bot", zapLogger.Error(err))
		return err
	}
	defer func() {
		err = discordBotEnv.Bot.Close()
		if err != nil {
			zap.Error("failed to close discord bot web socket", zapLogger.Error(err))
		}
	}()
	<-ctx.Done()

	zap.Info("Discord bot finished")
	if !taskScheduler.IsShutdown() {
		taskScheduler.Shutdown()
		zap.Info("scheduler finished")
	}
	return nil
}
