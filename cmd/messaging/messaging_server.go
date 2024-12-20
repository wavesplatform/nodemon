package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"nodemon/pkg/messaging"

	"go.uber.org/zap"

	"nodemon/pkg/tools"

	"github.com/nats-io/nats-server/v2/server"
)

const natsMaxPayloadSize int32 = 1024 * 1024 // 1 MB
const connectionsTimeoutDefault = 5 * server.AUTH_TIMEOUT

type natsConfig struct {
	serverAddress     string
	maxPayload        uint64
	logLevel          string
	development       bool
	connectionTimeout time.Duration
}

func parseNatsConfig() *natsConfig {
	c := new(natsConfig)
	tools.StringVarFlagWithEnv(&c.serverAddress, "nats-address",
		"127.0.0.1:4222", "NATS server address in form 'host:port'")
	tools.Uint64VarFlagWithEnv(&c.maxPayload, "nats-max-payload", uint64(natsMaxPayloadSize),
		"Max server payload size in bytes")
	tools.DurationVarFlagWithEnv(&c.connectionTimeout, "nats-connection-timeout", connectionsTimeoutDefault,
		"NATS connection timeout")
	tools.StringVarFlagWithEnv(&c.logLevel, "log-level", "INFO",
		"Logging level. Supported levels: DEBUG, INFO, WARN, ERROR, FATAL. Default logging level INFO.")
	tools.BoolVarFlagWithEnv(&c.development, "development", false, "Development mode.")
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

	shutdownFn, err := messaging.RunNatsMessagingServer(cfg.serverAddress, logger, cfg.maxPayload, cfg.connectionTimeout)
	if err != nil {
		log.Printf("Failed run nats messaging server: %v", err)
		return
	}
	defer shutdownFn()
	<-ctx.Done()
	logger.Info("NATS Server finished")
}
