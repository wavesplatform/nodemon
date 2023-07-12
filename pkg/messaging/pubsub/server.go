package pubsub

import (
	"context"
	"strings"

	"nodemon/pkg/entities"
	"nodemon/pkg/messaging"

	"github.com/pkg/errors"
	"go.nanomsg.org/mangos/v3/protocol"
	"go.nanomsg.org/mangos/v3/protocol/pub"
	_ "go.nanomsg.org/mangos/v3/transport/all" // registers all transports
	"go.uber.org/zap"
)

func StartPubMessagingServer(
	ctx context.Context,
	nanomsgURL string,
	alerts <-chan entities.Alert,
	logger *zap.Logger,
) error {
	if len(nanomsgURL) == 0 || len(strings.Fields(nanomsgURL)) > 1 {
		return errors.New("invalid nanomsg IPC URL for pub sub socket")
	}

	socketPub, sockErr := pub.NewSocket()
	if sockErr != nil {
		return sockErr
	}
	defer func(socketPub protocol.Socket) {
		if err := socketPub.Close(); err != nil {
			logger.Error("Failed to close pub socket", zap.Error(err))
		}
	}(socketPub)

	if err := socketPub.Listen(nanomsgURL); err != nil {
		return err
	}

	return enterLoop(ctx, alerts, logger, socketPub)
}

func enterLoop(ctx context.Context, alerts <-chan entities.Alert, logger *zap.Logger, socketPub protocol.Socket) error {
	for {
		select {
		case <-ctx.Done():
			return nil
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
			err = socketPub.Send(data)
			if err != nil {
				logger.Error("Failed to send alert to socket", zap.Error(err))
			}
		}
	}
}
