package pubsub

import (
	"context"

	"github.com/nats-io/nats.go"
	"github.com/pkg/errors"

	"go.uber.org/zap"

	"nodemon/pkg/entities"
	"nodemon/pkg/messaging"
)

func StartPubMessagingServer(
	ctx context.Context,
	natsPubSubURL string, // expected nats://host:port
	alerts <-chan entities.Alert,
	logger *zap.Logger,
	scheme string,
) error {
	nc, err := nats.Connect(natsPubSubURL)
	if err != nil {
		return err
	}
	loopErr := enterLoop(ctx, alerts, logger, nc, scheme)
	if loopErr != nil && !errors.Is(loopErr, context.Canceled) {
		return loopErr
	}
	return nil
}

func enterLoop(ctx context.Context, alerts <-chan entities.Alert,
	logger *zap.Logger, nc *nats.Conn, scheme string) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case alert := <-alerts:
			logger.Sugar().Infof("Alert has been generated: %v", alert)

			msg, err := messaging.NewAlertMessageFromAlert(alert)
			if err != nil {
				logger.Error("Failed to marshal an alert to json", zap.Error(err))
				continue
			}
			data, err := msg.MarshalBinary()
			if err != nil {
				logger.Error("Failed to marshal binary alert message", zap.Error(err))
				continue
			}
			topic := messaging.PubSubTopic + scheme
			err = nc.Publish(topic+string(alert.Type()), data)
			if err != nil {
				logger.Error("Failed to send alert to socket", zap.Error(err))
			}
		}
	}
}
