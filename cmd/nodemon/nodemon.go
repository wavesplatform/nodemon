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

	"nodemon/pkg/api"
	"nodemon/pkg/scraping"
	"nodemon/pkg/storing"
)

const (
	defaultNetworkTimeout  = 15 * time.Second
	defaultPollingInterval = 60 * time.Second
)

var (
	errorInvalidParameters = errors.New("invalid parameters")
)

func main() {
	err := run()
	switch err {
	case context.Canceled:
		os.Exit(130)
	case errorInvalidParameters:
		os.Exit(2)
	default:
		os.Exit(1)
	}
}

func run() error {
	var (
		storage     string
		nodes       string
		bindAddress string
		interval    time.Duration
		timeout     time.Duration
	)
	flag.StringVar(&storage, "storage", ".nodemon", "Path to storage. Default value is \".nodemon\"")
	flag.StringVar(&nodes, "nodes", "", "Initial list of Waves Blockchain nodes to monitor. Provide comma separated list of REST API URLs here.")
	flag.StringVar(&bindAddress, "bind", ":8080", "Local network address to bind the HTTP API of the service on. Default value is \":8080\".")
	flag.DurationVar(&interval, "interval", defaultPollingInterval, "Polling interval, seconds. Default value is 60")
	flag.DurationVar(&timeout, "timeout", defaultNetworkTimeout, "Network timeout, seconds. Default value is 15")
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

	ctx, done := signal.NotifyContext(context.Background(), os.Interrupt)
	defer done()

	ns, err := storing.NewNodesStorage(storage, nodes)
	if err != nil {
		log.Printf("Storage failure: %v", err)
		return err
	}
	defer func(cs *storing.NodesStorage) {
		err := cs.Close()
		if err != nil {
			log.Printf("Failed to close configuration storage: %v", err)
		}
	}(ns)

	a, err := api.NewAPI(bindAddress, ns)
	if err != nil {
		log.Printf("API failure: %v", err)
		return err
	}
	if err := a.Start(); err != nil {
		log.Printf("Failed to start API: %v", err)
		return err
	}

	scraper, err := scraping.NewScraper(ns, interval, timeout)
	if err != nil {
		log.Printf("ERROR: Failed to start monitoring: %v", err)
		return err
	}
	scraper.Start(ctx)

	<-ctx.Done()
	a.Shutdown()
	log.Println("Terminated")
	return nil
}
