package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"

	"nodemon/pkg/messaging"
	"nodemon/pkg/tools"

	"github.com/nats-io/nats-server/v2/server"
)

const NatsMaxPayloadSize int32 = 1024 * 1024 // 1 MB
const ConnectionsTimeoutDefault = 5 * server.AUTH_TIMEOUT

type natsConfig struct {
	serverURL                string
	maxPayload               int64
	logLevel                 string
	development              bool
	connectionTimeoutDefault time.Duration
}

func parseNatsConfig() *natsConfig {
	c := new(natsConfig)
	tools.StringVarFlagWithEnv(&c.serverURL, "nats-url",
		"nats://127.0.0.1:4222", "NATS server URL")
	tools.Int64VarFlagWithEnv(&c.maxPayload, "max-payload", int64(NatsMaxPayloadSize),
		"Max server payload size in bytes")
	tools.StringVarFlagWithEnv(&c.logLevel, "log-level", "INFO",
		"Logging level. Supported levels: DEBUG, INFO, WARN, ERROR, FATAL. Default logging level INFO.")
	tools.BoolVarFlagWithEnv(&c.development, "development", false, "Development mode.")
	tools.DurationVarFlagWithEnv(&c.connectionTimeoutDefault, "connection-timeout", ConnectionsTimeoutDefault,
		"HTTP API read timeout. Default value is 30s.")
	return c
}

func main() {
	ctx, done := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer done()

	cfg := parseNatsConfig()
	flag.Parse()

	logger, _, err := tools.SetupZapLogger(cfg.logLevel, cfg.development)
	if err != nil {
		log.Printf("Failed to setup zap logger: %v", err)
		return
	}
	defer func(zap *zap.Logger) {
		if syncErr := zap.Sync(); syncErr != nil {
			log.Println(syncErr)
		}
	}(logger)

	host, port, err := messaging.ParseHostAndPortFromURL(cfg.serverURL)
	if err != nil {
		logger.Fatal(fmt.Sprintf("failed to parse host and port %v", err))
	}

	if cfg.maxPayload > math.MaxInt32 || cfg.maxPayload < math.MinInt32 {
		logger.Fatal("max payload is too big or too small, must be in range of int32")
	}

	opts := &server.Options{
		MaxPayload: int32(cfg.maxPayload),
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
	if !s.ReadyForConnections(cfg.connectionTimeoutDefault) {
		logger.Fatal("NATS server is not ready for connections")
	}
	logger.Info(fmt.Sprintf("NATS Server is running on host %v, port %d", host, port))
	<-ctx.Done()

	logger.Info("NATS Server finished")
}
