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

	"nodemon/pkg/analysis/criteria"
	"nodemon/pkg/messaging/pair"
	"nodemon/pkg/messaging/pubsub"
	"nodemon/pkg/storing/specific"
	"nodemon/pkg/tools"

	zapLogger "go.uber.org/zap"

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
	errInvalidParameters = errors.New("invalid parameters")
)

func main() {
	const (
		contextCanceledExitCode   = 130
		invalidParametersExitCode = 2
	)
	if err := run(); err != nil {
		switch {
		case errors.Is(err, context.Canceled):
			os.Exit(contextCanceledExitCode)
		case errors.Is(err, errInvalidParameters):
			os.Exit(invalidParametersExitCode)
		default:
			log.Fatal(err)
		}
	}
}

type nodemonConfig struct {
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
	baseTargetThreshold    int
	logLevel               string
}

func newNodemonConfig() *nodemonConfig {
	c := new(nodemonConfig)
	flag.StringVar(&c.storage, "storage",
		".nodes.json", "Path to storage. Default value is \".nodes.json\"")
	flag.StringVar(&c.nodes, "nodes", "",
		"Initial list of Waves Blockchain nodes to monitor. Provide comma separated list of REST API URLs here.")
	flag.StringVar(&c.bindAddress, "bind", ":8080",
		"Local network address to bind the HTTP API of the service on. Default value is \":8080\".")
	flag.DurationVar(&c.interval, "interval",
		defaultPollingInterval, "Polling interval, seconds. Default value is 60")
	flag.DurationVar(&c.timeout, "timeout",
		defaultNetworkTimeout, "Network timeout, seconds. Default value is 15")
	flag.IntVar(&c.baseTargetThreshold, "base-target-threshold",
		0, "Base target threshold. Must be specified")
	flag.StringVar(&c.nanomsgPubSubURL, "nano-msg-pubsub-url",
		"ipc:///tmp/nano-msg-pubsub.ipc", "Nanomsg IPC URL for pubsub socket")
	flag.StringVar(&c.nanomsgPairTelegramURL, "nano-msg-pair-telegram-url",
		"", "Nanomsg IPC URL for pair socket")
	flag.StringVar(&c.nanomsgPairDiscordURL, "nano-msg-pair-discord-url",
		"", "Nanomsg IPC URL for pair socket")
	flag.DurationVar(&c.retention, "retention", defaultRetentionDuration,
		"Events retention duration. Default value is 12h")
	flag.DurationVar(&c.apiReadTimeout, "api-read-timeout", defaultAPIReadTimeout,
		"HTTP API read timeout. Default value is 30s.")
	flag.StringVar(&c.logLevel, "log-level", "INFO",
		"Logging level. Supported levels: DEBUG, INFO, WARN, ERROR, FATAL. Default logging level INFO.")
	return c
}

func (c *nodemonConfig) validate(zap *zapLogger.Logger) error {
	if len(c.storage) == 0 || len(strings.Fields(c.storage)) > 1 {
		zap.Error(fmt.Sprintf("Invalid storage path '%s'", c.storage))
		return errInvalidParameters
	}
	if c.interval <= 0 {
		zap.Error(fmt.Sprintf("Invalid polling interval '%s'", c.interval.String()))
		return errInvalidParameters
	}
	if c.timeout <= 0 {
		zap.Error(fmt.Sprintf("Invalid network timeout '%s'", c.timeout.String()))
		return errInvalidParameters
	}
	if c.retention <= 0 {
		zap.Error(fmt.Sprintf("Invalid retention duration '%s'", c.retention.String()))
		return errInvalidParameters
	}
	if c.baseTargetThreshold == 0 {
		zap.Error(fmt.Sprintf("Invalid base target threshold '%d'", c.baseTargetThreshold))
		return errInvalidParameters
	}
	return nil
}

func (c *nodemonConfig) runDiscordPairServer() bool { return c.nanomsgPairDiscordURL != "" }

func (c *nodemonConfig) runTelegramPairServer() bool { return c.nanomsgPairTelegramURL != "" }

func run() error {
	cfg := newNodemonConfig()
	flag.Parse()

	zap, atom, err := tools.SetupZapLogger(cfg.logLevel)
	if err != nil {
		log.Printf("Failed to setup zap logger: %v", err)
		return errInvalidParameters
	}
	defer func(zap *zapLogger.Logger) {
		if syncErr := zap.Sync(); syncErr != nil {
			log.Println(syncErr)
		}
	}(zap)

	if validateErr := cfg.validate(zap); validateErr != nil {
		return validateErr
	}

	ctx, done := signal.NotifyContext(context.Background(), os.Interrupt)
	defer done()

	ns, err := nodesStorage.NewJSONStorage(cfg.storage, strings.Fields(cfg.nodes), zap)
	if err != nil {
		zap.Error("failed to initialize nodes storage", zapLogger.Error(err))
		return err
	}
	defer func(cs nodesStorage.Storage) {
		if closeErr := cs.Close(); closeErr != nil {
			zap.Error("failed to close nodes storage", zapLogger.Error(closeErr))
		}
	}(ns)

	es, err := eventsStorage.NewStorage(cfg.retention, zap)
	if err != nil {
		zap.Error("failed to initialize events storage", zapLogger.Error(err))
		return err
	}
	defer func(es *eventsStorage.Storage) {
		if closeErr := es.Close(); closeErr != nil {
			zap.Error("failed to close events storage", zapLogger.Error(closeErr))
		}
	}(es)

	scraper, err := scraping.NewScraper(ns, es, cfg.interval, cfg.timeout, zap)
	if err != nil {
		zap.Error("failed to initialize scraper", zapLogger.Error(err))
		return err
	}

	privateNodesHandler, err := specific.NewPrivateNodesHandlerWithUnreachableInitialState(es, ns, zap)
	if err != nil {
		zap.Error("failed to create private nodes handler with unreachable initial state", zapLogger.Error(err))
		return err
	}

	notifications := scraper.Start(ctx)
	notifications = privateNodesHandler.Run(notifications) // wraps scrapper's notification with private nodes handler

	a, err := api.NewAPI(cfg.bindAddress, ns, es, cfg.apiReadTimeout, zap,
		privateNodesHandler.PrivateNodesEventsWriter(), atom,
	)
	if err != nil {
		zap.Error("failed to initialize API", zapLogger.Error(err))
		return err
	}
	if apiErr := a.Start(); apiErr != nil {
		zap.Error("failed to start API", zapLogger.Error(apiErr))
		return apiErr
	}

	opts := &analysis.AnalyzerOptions{
		BaseTargetCriterionOpts: &criteria.BaseTargetCriterionOptions{Threshold: cfg.baseTargetThreshold},
	}
	analyzer := analysis.NewAnalyzer(es, opts, zap)

	alerts := analyzer.Start(notifications)

	go func() {
		pubSubErr := pubsub.StartPubMessagingServer(ctx, cfg.nanomsgPubSubURL, alerts, zap)
		if pubSubErr != nil {
			zap.Fatal("failed to start pub messaging server", zapLogger.Error(pubSubErr))
		}
	}()

	if cfg.runTelegramPairServer() {
		go func() {
			pairErr := pair.StartPairMessagingServer(ctx, cfg.nanomsgPairTelegramURL, ns, es, zap)
			if pairErr != nil {
				zap.Fatal("failed to start pair messaging server", zapLogger.Error(pairErr))
			}
		}()
	}

	if cfg.runDiscordPairServer() {
		go func() {
			pairErr := pair.StartPairMessagingServer(ctx, cfg.nanomsgPairDiscordURL, ns, es, zap)
			if pairErr != nil {
				zap.Fatal("failed to start pair messaging server", zapLogger.Error(pairErr))
			}
		}()
	}

	<-ctx.Done()
	a.Shutdown()
	zap.Info("shutting down")
	return nil
}
