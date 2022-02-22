package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"nodemon/pkg/messaging"
	"os"
	"os/signal"
	"strings"
	"time"

	"nodemon/pkg/analysis"
	"nodemon/pkg/api"
	"nodemon/pkg/scraping"
	eventsStorage "nodemon/pkg/storing/events"
	nodesStorage "nodemon/pkg/storing/nodes"
)

const (
	defaultNetworkTimeout  = 15 * time.Second
	defaultPollingInterval = 60 * time.Second
)

const (
	defaultRetentionDuration = 12 * time.Hour
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
			os.Exit(1)
		}
	}
}

func run() error {
	var (
		storage     string
		nodes       string
		bindAddress string
		interval    time.Duration
		timeout     time.Duration
		nanomsgURL  string
		retention   time.Duration
	)
	flag.StringVar(&storage, "storage", ".nodemon", "Path to storage. Default value is \".nodemon\"")
	flag.StringVar(&nodes, "nodes", "", "Initial list of Waves Blockchain nodes to monitor. Provide comma separated list of REST API URLs here.")
	flag.StringVar(&bindAddress, "bind", ":8080", "Local network address to bind the HTTP API of the service on. Default value is \":8080\".")
	flag.DurationVar(&interval, "interval", defaultPollingInterval, "Polling interval, seconds. Default value is 60")
	flag.DurationVar(&timeout, "timeout", defaultNetworkTimeout, "Network timeout, seconds. Default value is 15")
	flag.StringVar(&nanomsgURL, "nano-msg-url", "ipc://:8000", "Nanomsg IPC URL. Default is tcp://:8000")
	flag.DurationVar(&retention, "retention", defaultRetentionDuration, "Events retention duration. Default value is 12h")
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

	a, err := api.NewAPI(bindAddress, ns)
	if err != nil {
		log.Printf("API failure: %v", err)
		return err
	}
	if err := a.Start(); err != nil {
		log.Printf("Failed to start API: %v", err)
		return err
	}

	scraper, err := scraping.NewScraper(ns, es, interval, timeout)
	if err != nil {
		log.Printf("ERROR: Failed to start monitoring: %v", err)
		return err
	}
	notifications := scraper.Start(ctx)

	analyzer := analysis.NewAnalyzer(es)

	alerts := analyzer.Start(notifications)

	socket, err := messaging.StartMessagingServer(nanomsgURL)
	if err != nil {
		log.Printf("Failed to start messaging server, %v", err)
	}
	go func() {
		for alert := range alerts {
			log.Printf("Alert has been generated: %v", alert)
			err := socket.Send([]byte(alert.Message()))
			if err != nil {
				log.Printf("failed to send a message to socket, %v", err)
			}
		}
	}()

	<-ctx.Done()
	a.Shutdown()
	log.Println("Terminated")
	return nil
}
