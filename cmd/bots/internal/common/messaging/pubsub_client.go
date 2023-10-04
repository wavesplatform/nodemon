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
		return errors.Wrap(sockErr, "failed to create new subscriber socket")
	}
	defer func() {
		_ = subSocket.Close() // can be ignored, only possible error is protocol.ErrClosed
	}()

	bot.SetSubSocket(subSocket)

	logger.Debug("Dialing the publisher socket", zap.String("url", nanomsgURL))
	if dialErr := subSocket.Dial(nanomsgURL); dialErr != nil {
		return errors.Wrapf(dialErr, "failed to dial '%s on sub socket'", nanomsgURL)
	}

	logger.Debug("Subscribing to all alert types")
	if err := bot.SubscribeToAllAlerts(); err != nil {
		return errors.Wrap(err, "failed to subscribe to all alerts")
	}

	logger.Info("Staring subscriber messaging service loop...")
	done := runSubLoop(ctx, nanomsgURL, subSocket, logger, bot)

	<-ctx.Done()
	logger.Info("Stopping subscriber messaging service...")
	<-done
	logger.Info("Subscriber messaging service finished")
	return nil
}

func runSubLoop(
	ctx context.Context,
	publisherURL string,
	subSocket protocol.Socket,
	logger *zap.Logger,
	bot Bot,
) <-chan struct{} {
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
		if ctx.Err() != nil {
			return
		}
		logger.Info("Subscriber messaging service is ready to receive messages from publisher",
			zap.String("publisher-url", publisherURL),
		)
		for {
			if ctx.Err() != nil {
				return
			}
			if err := recvMessage(subSocket, logger, bot); err != nil {
				if errors.Is(err, protocol.ErrClosed) { // socket is closed, this means that context is canceled
					return
				}
				logger.Error("failed to receive message", zap.Error(err))
			}
		}
	}(ch, sockCh)
	return ch
}

func recvMessage(subSocket protocol.Socket, logger *zap.Logger, bot Bot) error {
	logger.Debug("Subscriber service waiting for a new message...")
	msg, err := subSocket.Recv() // this operation is blocking, we have to close the socket to interrupt this block
	if err != nil {
		return errors.Wrap(err, "failed to receive message from sub socket")
	}
	logger.Debug("Subscriber service received a new message")
	alertMsg, err := messaging.NewAlertMessageFromBytes(msg)
	if err != nil {
		return errors.Wrap(err, "failed to parse alert message from bytes")
	}
	alertName, _ := alertMsg.AlertType().AlertName()
	logger.Debug("Subscriber service received a new alert",
		zap.Int("alert-type", int(alertMsg.AlertType())),
		zap.Stringer("alert-name", alertName),
		zap.Stringer("reference-id", alertMsg.ReferenceID()),
	)
	bot.SendAlertMessage(alertMsg)
	logger.Debug("Subscriber service sent received message to the bot service")
	return nil
}
