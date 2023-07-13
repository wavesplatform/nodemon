package messaging

import (
	"context"

	generalMessaging "nodemon/pkg/messaging"

	"go.nanomsg.org/mangos/v3/protocol"
	"go.nanomsg.org/mangos/v3/protocol/sub"
	_ "go.nanomsg.org/mangos/v3/transport/all" // registers all transports
	"go.uber.org/zap"
)

func StartSubMessagingClient(ctx context.Context, nanomsgURL string, bot Bot, logger *zap.Logger) error {
	subSocket, sockErr := sub.NewSocket()
	if sockErr != nil {
		return sockErr
	}
	defer func(subSocket protocol.Socket) {
		if closeErr := subSocket.Close(); closeErr != nil {
			logger.Error("failed to closed a sub socket", zap.Error(closeErr))
		}
	}(subSocket)

	bot.SetSubSocket(subSocket)

	if dialErr := subSocket.Dial(nanomsgURL); dialErr != nil {
		return dialErr
	}

	if err := bot.SubscribeToAllAlerts(); err != nil {
		return err
	}
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				msg, err := subSocket.Recv()
				if err != nil {
					logger.Error("failed to receive message", zap.Error(err))
					return
				}
				alertMsg, err := generalMessaging.NewAlertMessageFromBytes(msg)
				if err != nil {
					logger.Error("failed to parse alert message from bytes", zap.Error(err))
					return
				}
				bot.SendAlertMessage(alertMsg)
			}
		}
	}()

	<-ctx.Done()
	logger.Info("sub messaging service finished")
	return nil
}
