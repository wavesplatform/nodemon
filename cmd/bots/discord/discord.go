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
		log.Println("discord token is invalid")
		return common.ErrorInvalidParameters
	}

	if discordChatID == "" {
		log.Println("invalid discord chat ID")
		return common.ErrorInvalidParameters
	}

	ctx, done := signal.NotifyContext(context.Background(), os.Interrupt)
	defer done()

	discordBotEnv, err := initial.InitDiscordBot(discordBotToken, discordChatID)
	if err != nil {
		log.Println("failed to initialize discord bot")
		return errors.Wrap(err, "failed to init discord bot")
	}

	pairRequest := make(chan pairResponses.RequestPair)
	pairResponse := make(chan pairResponses.ResponsePair)
	handlers.InitDscHandlers(discordBotEnv, pairRequest, pairResponse)

	go func() {
		err := pubsub.StartSubMessagingClient(ctx, nanomsgPubSubURL, discordBotEnv)
		if err != nil {
			log.Printf("failed to start pubsub messaging service: %v", err)
			return
		}
	}()

	go func() {
		err := pair.StartPairMessagingClient(ctx, nanomsgPairUrl, pairRequest, pairResponse)
		if err != nil {
			log.Printf("failed to start pair messaging service: %v", err)
			return
		}
	}()

	taskScheduler := chrono.NewDefaultTaskScheduler()
	err = common.ScheduleNodesStatus(taskScheduler, pairRequest, pairResponse, discordBotEnv)
	if err != nil {
		taskScheduler.Shutdown()
		log.Printf("failed to schdule nodes status alert, %v", err)
		return err
	}
	log.Println("Nodes status alert has been scheduled successfully")

	err = discordBotEnv.Start()
	if err != nil {
		log.Printf("failed to start discord bot, %v", err)
		return err
	}
	defer func() {
		err = discordBotEnv.Bot.Close()
		if err != nil {
			log.Printf("failed to close discord web socket: %v", err)
		}
	}()
	<-ctx.Done()

	log.Println("Discord bot finished")

	if !taskScheduler.IsShutdown() {
		taskScheduler.Shutdown()
		log.Println("scheduler finished")
	}
	return nil
}
