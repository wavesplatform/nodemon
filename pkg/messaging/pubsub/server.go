package pubsub

import (
	"context"
	"fmt"
	"github.com/nats-io/nats.go"
	"github.com/pkg/errors"
	_ "go.nanomsg.org/mangos/v3/transport/all" // registers all transports
	"nodemon/pkg/entities"
	"nodemon/pkg/messaging"
	"strconv"
	"strings"

	"github.com/nats-io/nats-server/v2/server"
	"go.uber.org/zap"
)

const ConnectionsTimeoutDefault = 5 * server.AUTH_TIMEOUT
const pubsubTopic = "pubsub"

func parseHostAndPortFromUrl(natsPubSubUrl string) (string, int, error) {
	// Find the position of "://" and trim everything before it
	withoutProtocol := strings.SplitN(natsPubSubUrl, "://", 2)
	if len(withoutProtocol) != 2 {
		return "", 0, errors.New(fmt.Sprintf("failed to split the URL into pieces, URL: %s", natsPubSubUrl))
	}

	// Split the remaining part into host and port
	hostPort := strings.Split(withoutProtocol[1], ":")
	if len(hostPort) != 2 {
		return "", 0, errors.New(fmt.Sprintf("failed to split host port string into host and port, %s", hostPort))
	}

	host := hostPort[0]
	// Convert the port to an integer
	port, err := strconv.Atoi(hostPort[1])
	if err != nil {
		return "", 0, errors.Errorf("failed to parse port, %v", err)
	}
	return host, port, nil
}

func StartPubMessagingServer(
	ctx context.Context,
	maxPayloadSize int32,
	natsPubSubUrl string, // expected nats://host:port
	alerts <-chan entities.Alert,
	logger *zap.Logger,
) error {
	if len(natsPubSubUrl) == 0 {
		return errors.New("invalid nats pubsub URL")
	}
	host, port, err := parseHostAndPortFromUrl(natsPubSubUrl)
	if err != nil {
		return err
	}

	opts := &server.Options{
		MaxPayload: maxPayloadSize,
		Host:       host,
		Port:       port,
	}
	s, err := server.NewServer(opts)
	if err != nil {
		return errors.Errorf("failed to create NATS server: %v", err)
	}
	go s.Start()

	if !s.ReadyForConnections(ConnectionsTimeoutDefault) {
		return errors.Errorf("NATS Server not ready for connections")
	}
	logger.Info("NATS PubSub Server is running...")

	socket, err := nats.Connect(fmt.Sprintf("nats://%s:%d", host, port))

	loopErr := enterLoop(ctx, alerts, logger, socket)
	if loopErr != nil && !errors.Is(loopErr, context.Canceled) {
		return loopErr
	}
	return nil
}

func enterLoop(ctx context.Context, alerts <-chan entities.Alert, logger *zap.Logger, socket *nats.Conn) error {
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
			err = socket.Publish(pubsubTopic, data)
			if err != nil {
				logger.Error("Failed to send alert to socket", zap.Error(err))
			}
		}
	}
}
