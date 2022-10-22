package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"os"
	"os/signal"
	"strings"
	"time"

	"nodemon/pkg/analysis/criteria"
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
		switch err {
		case context.Canceled:
			os.Exit(130)
		case errorInvalidParameters:
			os.Exit(2)
		default:
			log.Println(err)
			os.Exit(1)
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
		baseTargetThreshold    int
	)
	flag.StringVar(&storage, "storage", ".nodemon", "Path to storage. Default value is \".nodemon\"")
	flag.StringVar(&nodes, "nodes", "", "Initial list of Waves Blockchain nodes to monitor. Provide comma separated list of REST API URLs here.")
	flag.StringVar(&bindAddress, "bind", ":8080", "Local network address to bind the HTTP API of the service on. Default value is \":8080\".")
	flag.DurationVar(&interval, "interval", defaultPollingInterval, "Polling interval, seconds. Default value is 60")
	flag.DurationVar(&timeout, "timeout", defaultNetworkTimeout, "Network timeout, seconds. Default value is 15")
	flag.IntVar(&baseTargetThreshold, "base-target-threshold", 0, "Base target threshold. Must be specified")
	flag.StringVar(&nanomsgPubSubURL, "nano-msg-pubsub-url", "ipc:///tmp/nano-msg-pubsub.ipc", "Nanomsg IPC URL for pubsub socket")
	flag.StringVar(&nanomsgPairTelegramURL, "nano-msg-pair-telegram-url", "", "Nanomsg IPC URL for pair socket")
	flag.StringVar(&nanomsgPairDiscordURL, "nano-msg-pair-discord-url", "", "Nanomsg IPC URL for pair socket")
	flag.DurationVar(&retention, "retention", defaultRetentionDuration, "Events retention duration. Default value is 12h")
	flag.DurationVar(&apiReadTimeout, "api-read-timeout", defaultAPIReadTimeout, "HTTP API read timeout. Default value is 30s.")
	flag.Parse()

	if len(storage) == 0 || len(strings.Fields(storage)) > 1 {
		log.Printf("Invalid storage path '%s'", storage)
		return errorInvalidParameters
	}
	if interval <= 0 {
		log.Printf("Invalid polling interval '%s'", interval.String())
		return errorInvalidParameters
	}
	if timeout <= 0 {
		log.Printf("Invalid network timout '%s'", timeout.String())
		return errorInvalidParameters
	}
	if retention <= 0 {
		log.Printf("Invalid retention duration '%s'", retention.String())
		return errorInvalidParameters
	}
	if baseTargetThreshold == 0 {
		log.Printf("Invalid base target threshold '%d'", baseTargetThreshold)
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

	ns, err := nodesStorage.NewStorage(storage, nodes)
	if err != nil {
		log.Printf("Nodes storage failure: %v", err)
		return err
	}
	defer func(cs *nodesStorage.Storage) {
		err := cs.Close()
		if err != nil {
			log.Printf("Failed to close nodes storage: %v", err)
		}
	}(ns)

	es, err := eventsStorage.NewStorage(retention)
	if err != nil {
		log.Printf("Events storage failure: %v", err)
		return err
	}
	defer func(es *eventsStorage.Storage) {
		if err := es.Close(); err != nil {
			log.Printf("Failed to close events storage: %v", err)
		}
	}(es)

	scraper, err := scraping.NewScraper(ns, es, interval, timeout)
	if err != nil {
		log.Printf("ERROR: Failed to start monitoring: %v", err)
		return err
	}
	notifications, specificNodesTs := scraper.Start(ctx)

	a, err := api.NewAPI(bindAddress, ns, es, specificNodesTs, apiReadTimeout)
	if err != nil {
		log.Printf("API failure: %v", err)
		return err
	}
	if err := a.Start(); err != nil {
		log.Printf("Failed to start API: %v", err)
		return err
	}

	opts := &analysis.AnalyzerOptions{
		BaseTargetCriterionOpts: &criteria.BaseTargetCriterionOptions{Threshold: baseTargetThreshold},
	}
	analyzer := analysis.NewAnalyzer(es, opts)

	alerts := analyzer.Start(notifications)

	go func() {
		err := pubsub.StartPubMessagingServer(ctx, nanomsgPubSubURL, alerts)
		if err != nil {
			log.Printf("failed to start pair messaging service: %v", err)
		}
	}()

	if runTelegramPairServer {
		go func() {
			err := pair.StartPairMessagingServer(ctx, nanomsgPairTelegramURL, ns, es)
			if err != nil {
				log.Printf("failed to start pair messaging service: %v", err)
			}
		}()
	}

	if runDiscordPairServer {
		go func() {
			err := pair.StartPairMessagingServer(ctx, nanomsgPairDiscordURL, ns, es)
			if err != nil {
				log.Printf("failed to start pair messaging service: %v", err)
			}
		}()
	}

	<-ctx.Done()
	a.Shutdown()
	log.Println("Terminated")
	return nil
}
