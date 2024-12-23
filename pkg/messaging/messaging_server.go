package messaging

import (
	"fmt"
	"math"
	"net"
	"strconv"
	"time"

	"go.uber.org/zap"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/pkg/errors"
)

func RunNatsMessagingServer( //nolint:nonamedreturns // needs in defer
	serverAddress string,
	logger *zap.Logger,
	maxPayload uint64,
	connectionTimeout time.Duration,
) (_ func(), runErr error) {
	host, portString, err := net.SplitHostPort(serverAddress)
	if err != nil {
		return nil, errors.Errorf("failed to parse host and port: %v", err)
	}

	port, err := strconv.Atoi(portString)
	if err != nil {
		return nil, errors.Errorf("failed to parse port from the URL: %v", err)
	}
	if port <= 0 || port > math.MaxUint16 {
		return nil, errors.Errorf("invalid port number (%d)", port)
	}

	if connectionTimeout <= 0 {
		return nil, errors.Errorf("connection timeout must be positive")
	}

	if maxPayload > math.MaxInt32 {
		return nil, errors.Errorf("max payload is too big, must be in range of int32")
	}

	opts := &server.Options{
		MaxPayload: int32(maxPayload),
		Host:       host,
		Port:       port,
		NoSigs:     true,
	}
	s, err := server.NewServer(opts)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create NATS server")
	}
	s.Start() // this will not block
	defer func() {
		if runErr != nil { // if there was an error, we need to shut down the server
			s.Shutdown()
		}
	}()
	if !s.ReadyForConnections(connectionTimeout) {
		return nil, errors.New("NATS server is not ready for connections")
	}
	logger.Info(fmt.Sprintf("NATS Server is running on %s:%d", host, port))
	return s.Shutdown, nil
}
