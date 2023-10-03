package messaging

import (
	"context"

	"github.com/pkg/errors"
	"go.nanomsg.org/mangos/v3/protocol"
	"go.nanomsg.org/mangos/v3/protocol/sub"
	_ "go.nanomsg.org/mangos/v3/transport/all" // registers all transports
	"go.uber.org/zap"

	"nodemon/pkg/messaging"
)

func StartSubMessagingClient(ctx context.Context, nanomsgURL string, bot Bot, logger *zap.Logger) error {
	subSocket, sockErr := sub.NewSocket()
	if sockErr != nil {
		return sockErr
	}
	defer func() {
		_ = subSocket.Close() // can be ignored, only possible error is protocol.ErrClosed
	}()

	bot.SetSubSocket(subSocket)

	if dialErr := subSocket.Dial(nanomsgURL); dialErr != nil {
		return errors.Wrapf(dialErr, "failed to dial '%s on sub socket'", nanomsgURL)
	}

	if err := bot.SubscribeToAllAlerts(); err != nil {
		return errors.Wrap(err, "failed to subscribe to all alerts")
	}

	done := runSubLoop(ctx, subSocket, logger, bot)

	<-ctx.Done()
	logger.Info("stopping sub messaging service...")
	<-done
	logger.Info("sub messaging service finished")
	return nil
}

func runSubLoop(ctx context.Context, subSocket protocol.Socket, logger *zap.Logger, bot Bot) <-chan struct{} {
	sockCh := make(chan struct{})
	go func() { // run socket closer goroutine
		defer close(sockCh)
		<-ctx.Done()
		_ = subSocket.Close() // can be ignored, only possible error is protocol.ErrClosed
	}()
	ch := make(chan struct{})
	go func(done chan<- struct{}, closedSock <-chan struct{}) {
		defer func() {
			<-closedSock
			close(done)
		}()
		for {
			if ctx.Err() != nil {
				return
			}
			if err := recvMessage(subSocket, bot); err != nil {
				if errors.Is(err, protocol.ErrClosed) { // socket is closed, this means that context is canceled
					return
				}
				logger.Error("failed to receive message", zap.Error(err))
			}
		}
	}(ch, sockCh)
	return ch
}

func recvMessage(subSocket protocol.Socket, bot Bot) error {
	msg, err := subSocket.Recv() // this operation is blocking, we have to close the socket to interrupt this block
	if err != nil {
		return errors.Wrap(err, "failed to receive message from sub socket")
	}
	alertMsg, err := messaging.NewAlertMessageFromBytes(msg)
	if err != nil {
		return errors.Wrap(err, "failed to parse alert message from bytes")
	}
	bot.SendAlertMessage(alertMsg)
	return nil
}
