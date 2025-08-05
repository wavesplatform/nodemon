package messaging

import (
	"context"
	"log/slog"

	"github.com/nats-io/nats.go"
	"github.com/pkg/errors"

	"nodemon/pkg/messaging"
	"nodemon/pkg/tools/logging/attrs"
)

func StartSubMessagingClient(ctx context.Context, natsServerURL string, bot Bot, logger *slog.Logger) error {
	// Connect to a NATS server
	nc, err := nats.Connect(natsServerURL, nats.Timeout(nats.DefaultTimeout))
	if err != nil {
		logger.Error("Failed to connect to nats server", attrs.Error(err))
		return err
	}
	defer nc.Close()
	bot.SetNatsConnection(nc)
	alertHandlerFunc := func(msg *nats.Msg) {
		hndlErr := handleReceivedMessage(msg.Data, bot)
		if hndlErr != nil {
			logger.Error("failed to handle received message from pubsub server", attrs.Error(hndlErr))
		}
	}
	bot.SetAlertHandlerFunc(alertHandlerFunc)

	if subscrErr := bot.SubscribeToAllAlerts(); subscrErr != nil {
		return subscrErr
	}

	logger.Info("sub messaging service started", slog.String("natsServerURL", natsServerURL))
	<-ctx.Done()
	logger.Info("stopping sub messaging service...")
	logger.Info("sub messaging service finished")
	return nil
}

func handleReceivedMessage(msg []byte, bot Bot) error {
	alertMsg, err := messaging.NewAlertMessageFromBytes(msg)
	if err != nil {
		return errors.Wrap(err, "failed to parse alert message from bytes")
	}
	bot.SendAlertMessage(alertMsg)
	return nil
}
