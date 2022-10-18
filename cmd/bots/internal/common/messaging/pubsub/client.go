package pubsub

import (
	"context"

	"go.nanomsg.org/mangos/v3/protocol"
	"go.nanomsg.org/mangos/v3/protocol/sub"
	_ "go.nanomsg.org/mangos/v3/transport/all"
	"go.uber.org/zap"
	"nodemon/cmd/bots/internal/common/messaging"
)

func StartSubMessagingClient(ctx context.Context, nanomsgURL string, bot messaging.Bot, logger *zap.Logger) error {
	subSocket, err := sub.NewSocket()
	if err != nil {
		return err
	}
	defer func(subSocket protocol.Socket) {
		if err := subSocket.Close(); err != nil {
			logger.Error("failed to closed a sub socket", zap.Error(err))
		}
	}(subSocket)

	bot.SetSubSocket(subSocket)

	if err := subSocket.Dial(nanomsgURL); err != nil {
		return err
	}

	err = bot.SubscribeToAllAlerts()
	if err != nil {
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
				bot.SendAlertMessage(msg)
			}
		}
	}()

	<-ctx.Done()
	logger.Info("sub messaging service finished")
	return nil
}
