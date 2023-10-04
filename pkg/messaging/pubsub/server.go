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
		return errors.Wrap(sockErr, "failed to create new publisher socket")
	}
	defer func(socketPub protocol.Socket) {
		if err := socketPub.Close(); err != nil {
			logger.Error("Failed to close pub socket", zap.Error(err))
		}
	}(socketPub)

	logger.Debug("Publisher messaging service start listening", zap.String("listen", nanomsgURL))
	if err := socketPub.Listen(nanomsgURL); err != nil {
		return errors.Wrapf(err, "publisher socket failed to start listening on '%s'", nanomsgURL)
	}

	logger.Info("Staring publisher messaging service loop...")
	return enterLoop(ctx, alerts, logger, nanomsgURL, socketPub)
}

func enterLoop(
	ctx context.Context,
	alerts <-chan entities.Alert,
	logger *zap.Logger,
	listen string,
	socketPub protocol.Socket,
) error {
	logger.Info("Publisher messaging service is ready to send messages to subscribers",
		zap.String("listen", listen),
	)
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
