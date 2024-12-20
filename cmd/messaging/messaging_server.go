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
	serverURL         string
	maxPayload        uint64
	logLevel          string
	development       bool
	connectionTimeout time.Duration
}

func parseNatsConfig() *natsConfig {
	c := new(natsConfig)
	tools.StringVarFlagWithEnv(&c.serverURL, "nats-url",
		"nats://127.0.0.1:4222", "NATS server URL")
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

	err = messaging.RunNatsMessagingServer(cfg.serverURL, logger, cfg.maxPayload, cfg.connectionTimeout)
	if err != nil {
		log.Printf("Failed run nats messaging server: %v", err)
		return
	}
	<-ctx.Done()
	logger.Info("NATS Server finished")
}
