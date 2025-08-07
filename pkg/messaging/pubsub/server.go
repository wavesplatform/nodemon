package pubsub

import (
	"context"
	"log/slog"

	"github.com/nats-io/nats.go"
	"github.com/pkg/errors"

	"nodemon/pkg/entities"
	"nodemon/pkg/messaging"
	"nodemon/pkg/tools/logging/attrs"
)

func StartPubMessagingServer(
	ctx context.Context,
	natsPubSubURL string, // expected nats://host:port
	alerts <-chan entities.Alert,
	logger *slog.Logger,
	scheme string,
) error {
	nc, err := nats.Connect(natsPubSubURL)
	if err != nil {
		return err
	}
	defer nc.Close()
	loopErr := enterLoop(ctx, alerts, logger, nc, scheme)
	if loopErr != nil && !errors.Is(loopErr, context.Canceled) {
		return loopErr
	}
	return nil
}

func enterLoop(ctx context.Context, alerts <-chan entities.Alert,
	logger *slog.Logger, nc *nats.Conn, scheme string) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case alert := <-alerts:
			logger.Info("Alert has been generated", slog.Any("alert", alert))

			msg, err := messaging.NewAlertMessageFromAlert(alert)
			if err != nil {
				logger.Error("Failed to marshal an alert to json", attrs.Error(err))
				continue
			}
			data, err := msg.MarshalBinary()
			if err != nil {
				logger.Error("Failed to marshal binary alert message", attrs.Error(err))
				continue
			}
			topic := messaging.PubSubMsgTopic(scheme, alert.Type())
			err = nc.Publish(topic, data)
			if err != nil {
				logger.Error("Failed to send alert to socket", attrs.Error(err))
			}
		}
	}
}
