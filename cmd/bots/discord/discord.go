package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"

	"github.com/pkg/errors"
	"github.com/procyon-projects/chrono"
	zapLogger "go.uber.org/zap"
	"nodemon/cmd/bots/internal/common"
	initial "nodemon/cmd/bots/internal/common/init"
	"nodemon/cmd/bots/internal/common/messaging/pair"
	"nodemon/cmd/bots/internal/common/messaging/pubsub"
	"nodemon/cmd/bots/internal/discord/handlers"
	pairResponses "nodemon/pkg/messaging/pair"
)

func main() {
	err := runDiscordBot()
	if err != nil {
		switch err {
		case context.Canceled:
			os.Exit(130)
		default:
			log.Println(err)
			os.Exit(1)
		}
	}
}

func runDiscordBot() error {
	zap, err := zapLogger.NewDevelopment()
	if err != nil {
		log.Fatalf("can't initialize zap logger: %v", err)
	}
	defer func(zap *zapLogger.Logger) {
		err := zap.Sync()
		if err != nil {
			log.Println(err)
		}
	}(zap)

	var (
		nanomsgPubSubURL string
		nanomsgPairUrl   string
		discordBotToken  string
		discordChatID    string
	)
	flag.StringVar(&nanomsgPubSubURL, "nano-msg-pubsub-url", "ipc:///tmp/discord/nano-msg-nodemon-pubsub.ipc", "Nanomsg IPC URL for pubsub socket")
	flag.StringVar(&nanomsgPairUrl, "nano-msg-pair-discord-url", "ipc:///tmp/nano-msg-nodemon-pair.ipc", "Nanomsg IPC URL for pair socket")
	flag.StringVar(&discordBotToken, "discord-bot-token", "", "The secret token used to authenticate the bot")
	flag.StringVar(&discordChatID, "discord-chat-id", "", "discord chat ID to send alerts through")
	flag.Parse()

	if discordBotToken == "" {
		zap.Error("discord bot token is required")
		return common.ErrorInvalidParameters
	}

	if discordChatID == "" {
		zap.Error("discord chat ID is required")
		return common.ErrorInvalidParameters
	}

	ctx, done := signal.NotifyContext(context.Background(), os.Interrupt)
	defer done()

	discordBotEnv, err := initial.InitDiscordBot(discordBotToken, discordChatID, zap)
	if err != nil {
		return errors.Wrap(err, "failed to init discord bot")
	}

	pairRequest := make(chan pairResponses.RequestPair)
	pairResponse := make(chan pairResponses.ResponsePair)
	handlers.InitDscHandlers(discordBotEnv, pairRequest, pairResponse)

	go func() {
		err := pubsub.StartSubMessagingClient(ctx, nanomsgPubSubURL, discordBotEnv, zap)
		if err != nil {
			zap.Fatal("failed to start sub messaging client", zapLogger.Error(err))
			return
		}
	}()

	go func() {
		err := pair.StartPairMessagingClient(ctx, nanomsgPairUrl, pairRequest, pairResponse, zap)
		if err != nil {
			zap.Fatal("failed to start pair messaging client", zapLogger.Error(err))
			return
		}
	}()

	taskScheduler := chrono.NewDefaultTaskScheduler()
	err = common.ScheduleNodesStatus(taskScheduler, pairRequest, pairResponse, discordBotEnv, zap)
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
