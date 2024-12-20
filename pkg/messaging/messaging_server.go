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

func RunNatsMessagingServer(
	serverURL string,
	logger *zap.Logger,
	maxPayload uint64,
	connectionTimeout time.Duration,
) error {
	host, portString, err := net.SplitHostPort(serverURL)
	if err != nil {
		return errors.Errorf("failed to parse host and port: %v", err)
	}

	port, err := strconv.Atoi(portString)
	if err != nil {
		return errors.Errorf("failed to parse port from the URL: %v", err)
	}
	if port <= 0 || port > math.MaxUint16 {
		return errors.Errorf("invalid port number (%d)", port)
	}

	if connectionTimeout <= 0 {
		return errors.Errorf("connection timeout must be positive")
	}

	if maxPayload > math.MaxInt32 {
		return errors.Errorf("max payload is too big, must be in range of int32")
	}

	opts := &server.Options{
		MaxPayload: int32(maxPayload),
		Host:       host,
		Port:       port,
		NoSigs:     true,
	}
	s, err := server.NewServer(opts)
	if err != nil {
		logger.Fatal(fmt.Sprintf("failed to create NATS server, %v", err))
	}
	go s.Start()
	defer func() {
		s.Shutdown()
		s.WaitForShutdown()
	}()
	if !s.ReadyForConnections(connectionTimeout) {
		logger.Fatal("NATS server is not ready for connections")
	}
	logger.Info(fmt.Sprintf("NATS Server is running on %s:%d", host, port))
	return nil
}
