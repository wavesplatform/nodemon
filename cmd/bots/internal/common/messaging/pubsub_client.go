package messaging

import (
	"context"
	"github.com/nats-io/nats.go"

	"github.com/pkg/errors"
	_ "go.nanomsg.org/mangos/v3/transport/all" // registers all transports
	"go.uber.org/zap"

	"nodemon/pkg/messaging"
)

const pubSubTopic = "pubsub"

func StartSubMessagingClient(ctx context.Context, natsServerURL string, bot Bot, logger *zap.Logger) error {

	// Connect to a NATS server
	nc, err := nats.Connect(natsServerURL)
	if err != nil {
		zap.S().Fatalf("Failed to connect to nats server: %v", err)
		return err
	}
	defer nc.Close()

	_, err = nc.Subscribe(pubSubTopic, func(msg *nats.Msg) {
		err := handleReceivedMessage(msg.Data, bot)
		if err != nil {
			zap.S().Errorf("failed to handle received message from pubsub server %v", err)
		}
	})
	if err != nil {
		zap.S().Fatalf("Failed to subscribe to block updates: %v", err)
		return err
	}
	if err := bot.SubscribeToAllAlerts(); err != nil {
		return err
	}

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
