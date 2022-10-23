package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"time"

	gow "github.com/wavesplatform/gowaves/pkg/util/common"
	zapLogger "go.uber.org/zap"
	"nodemon/pkg/messaging/pair"
	"nodemon/pkg/messaging/pubsub"

	"nodemon/pkg/analysis"
	"nodemon/pkg/api"
	"nodemon/pkg/scraping"
	eventsStorage "nodemon/pkg/storing/events"
	nodesStorage "nodemon/pkg/storing/nodes"
)

const (
	defaultNetworkTimeout    = 15 * time.Second
	defaultPollingInterval   = 60 * time.Second
	defaultRetentionDuration = 12 * time.Hour
	defaultAPIReadTimeout    = 30 * time.Second
)

var (
	errorInvalidParameters = errors.New("invalid parameters")
)

func main() {

	if err := run(); err != nil {
		switch {
		case errors.Is(err, context.Canceled):
			os.Exit(130)
		case errors.Is(err, errorInvalidParameters):
			os.Exit(2)
		default:
			log.Fatal(err)
		}
	}
}

func run() error {
	var (
		storage                string
		nodes                  string
		bindAddress            string
		interval               time.Duration
		timeout                time.Duration
		nanomsgPubSubURL       string
		nanomsgPairTelegramURL string
		nanomsgPairDiscordURL  string
		retention              time.Duration
		apiReadTimeout         time.Duration
		logLevel               string
	)
	flag.StringVar(&storage, "storage", ".nodemon", "Path to storage. Default value is \".nodemon\"")
	flag.StringVar(&nodes, "nodes", "", "Initial list of Waves Blockchain nodes to monitor. Provide comma separated list of REST API URLs here.")
	flag.StringVar(&bindAddress, "bind", ":8080", "Local network address to bind the HTTP API of the service on. Default value is \":8080\".")
	flag.DurationVar(&interval, "interval", defaultPollingInterval, "Polling interval, seconds. Default value is 60")
	flag.DurationVar(&timeout, "timeout", defaultNetworkTimeout, "Network timeout, seconds. Default value is 15")
	flag.StringVar(&nanomsgPubSubURL, "nano-msg-pubsub-url", "ipc:///tmp/nano-msg-pubsub.ipc", "Nanomsg IPC URL for pubsub socket")
	flag.StringVar(&nanomsgPairTelegramURL, "nano-msg-pair-telegram-url", "", "Nanomsg IPC URL for pair socket")
	flag.StringVar(&nanomsgPairDiscordURL, "nano-msg-pair-discord-url", "", "Nanomsg IPC URL for pair socket")
	flag.DurationVar(&retention, "retention", defaultRetentionDuration, "Events retention duration. Default value is 12h")
	flag.DurationVar(&apiReadTimeout, "api-read-timeout", defaultAPIReadTimeout, "HTTP API read timeout. Default value is 30s.")
	flag.StringVar(&logLevel, "log-level", "INFO", "Logging level. Supported levels: DEBUG, INFO, WARN, ERROR, FATAL. Default logging level INFO.")
	flag.Parse()

	zap, _ := gow.SetupLogger(logLevel)
	defer func(zap *zapLogger.Logger) {
		if err := zap.Sync(); err != nil {
			log.Println(err)
		}
	}(zap)

	if len(storage) == 0 || len(strings.Fields(storage)) > 1 {
		zap.Error(fmt.Sprintf("Invalid storage path '%s'", storage))
		return errorInvalidParameters
	}
	if interval <= 0 {
		zap.Error(fmt.Sprintf("Invalid polling interval '%s'", interval.String()))
		return errorInvalidParameters
	}
	if timeout <= 0 {
		zap.Error(fmt.Sprintf("Invalid network timeout '%s'", timeout.String()))
		return errorInvalidParameters
	}
	if retention <= 0 {
		zap.Error(fmt.Sprintf("Invalid retention duration '%s'", retention.String()))
		return errorInvalidParameters
	}
	var (
		runDiscordPairServer  bool
		runTelegramPairServer bool
	)

	if nanomsgPairTelegramURL != "" {
		runTelegramPairServer = true
	}
	if nanomsgPairDiscordURL != "" {
		runDiscordPairServer = true
	}

	ctx, done := signal.NotifyContext(context.Background(), os.Interrupt)
	defer done()

	ns, err := nodesStorage.NewStorage(storage, nodes, zap)
	if err != nil {
		zap.Error("failed to initialize nodes storage", zapLogger.Error(err))
		return err
	}
	defer func(cs *nodesStorage.Storage) {
		err := cs.Close()
		if err != nil {
			zap.Error("failed to close nodes storage", zapLogger.Error(err))
		}
	}(ns)

	es, err := eventsStorage.NewStorage(retention, zap)
	if err != nil {
		zap.Error("failed to initialize events storage", zapLogger.Error(err))
		return err
	}
	defer func(es *eventsStorage.Storage) {
		if err := es.Close(); err != nil {
			zap.Error("failed to close events storage", zapLogger.Error(err))
		}
	}(es)

	scraper, err := scraping.NewScraper(ns, es, interval, timeout, zap)
	if err != nil {
		zap.Error("failed to initialize scraper", zapLogger.Error(err))
		return err
	}
	notifications, specificNodesTs := scraper.Start(ctx)

	a, err := api.NewAPI(bindAddress, ns, es, specificNodesTs, apiReadTimeout, zap)
	if err != nil {
		zap.Error("failed to initialize API", zapLogger.Error(err))
		return err
	}
	if err := a.Start(); err != nil {
		zap.Error("failed to start API", zapLogger.Error(err))
		return err
	}

	analyzer := analysis.NewAnalyzer(es, nil, zap)

	alerts := analyzer.Start(notifications)

	go func() {
		err := pubsub.StartPubMessagingServer(ctx, nanomsgPubSubURL, alerts, zap)
		if err != nil {
			zap.Fatal("failed to start pub messaging server", zapLogger.Error(err))
		}
	}()

	if runTelegramPairServer {
		go func() {
			err := pair.StartPairMessagingServer(ctx, nanomsgPairTelegramURL, ns, es, zap)
			if err != nil {
				zap.Fatal("failed to start pair messaging server", zapLogger.Error(err))
			}
		}()
	}

	if runDiscordPairServer {
		go func() {
			err := pair.StartPairMessagingServer(ctx, nanomsgPairDiscordURL, ns, es, zap)
			if err != nil {
				zap.Fatal("failed to start pair messaging server", zapLogger.Error(err))
			}
		}()
	}

	<-ctx.Done()
	a.Shutdown()
	zap.Info("shutting down")
	return nil
}
