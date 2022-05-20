package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"log"
	"os"
	"os/signal"
	"strings"
	"time"

	"go.nanomsg.org/mangos/v3/protocol"
	"nodemon/pkg/messaging/pair"
	"nodemon/pkg/messaging/pubsub"

	"nodemon/pkg/analysis"
	"nodemon/pkg/api"
	"nodemon/pkg/messaging"
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
		storage          string
		nodes            string
		bindAddress      string
		interval         time.Duration
		timeout          time.Duration
		nanomsgPubSubURL string
		nanomsgPairURL   string
		retention        time.Duration
	)
	flag.StringVar(&storage, "storage", ".nodemon", "Path to storage. Default value is \".nodemon\"")
	flag.StringVar(&nodes, "nodes", "", "Initial list of Waves Blockchain nodes to monitor. Provide comma separated list of REST API URLs here.")
	flag.StringVar(&bindAddress, "bind", ":8080", "Local network address to bind the HTTP API of the service on. Default value is \":8080\".")
	flag.DurationVar(&interval, "interval", defaultPollingInterval, "Polling interval, seconds. Default value is 60")
	flag.DurationVar(&timeout, "timeout", defaultNetworkTimeout, "Network timeout, seconds. Default value is 15")
	flag.StringVar(&nanomsgPubSubURL, "nano-msg-pubsub-url", "ipc:///tmp/nano-msg-nodemon-pubsub.ipc", "Nanomsg IPC URL for pubsub socket")
	flag.StringVar(&nanomsgPairURL, "nano-msg-pair-url", "ipc:///tmp/nano-msg-nodemon-pair.ipc", "Nanomsg IPC URL for pair socket")
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

	analyzer := analysis.NewAnalyzer(es, nil)

	alerts := analyzer.Start(notifications)

	socketPubSub, err := pubsub.StartPubSubMessagingServer(nanomsgPubSubURL)
	if err != nil {
		log.Printf("Failed to start messaging server: %v", err)
		return err
	}
	defer func(socketPubSub protocol.Socket) {
		if err := socketPubSub.Close(); err != nil {
			log.Printf("Failed to close pubsub socket: %v", err)
		}
	}(socketPubSub)
	go func() { // pubsub messaging
		for alert := range alerts {
			log.Printf("Alert has been generated: %v", alert)

			jsonAlert, err := json.Marshal(
				messaging.Alert{
					AlertDescription: alert.ShortDescription(),
					Severity:         alert.Severity(),
					Details:          alert.Message(),
				})
			if err != nil {
				log.Printf("failed to marshal alert to json, %v", err)
			}

			message := &bytes.Buffer{}
			message.WriteByte(byte(alert.Type()))
			message.Write(jsonAlert)
			err = socketPubSub.Send(message.Bytes())
			if err != nil {
				log.Printf("failed to send a message to socket, %v", err)
			}
		}
	}()

	socketPair, err := pair.StartPairMessagingServer(nanomsgPairURL)
	if err != nil {
		log.Printf("Failed to start pair messaging server: %v", err)
		return err
	}
	defer func(socketPair protocol.Socket) {
		if err := socketPair.Close(); err != nil {
			log.Printf("Failed to close pair socket: %v", err)
		}
	}(socketPair)
	go func() { // pair messaging
		for {
			select {
			case <-ctx.Done():
				return
			default:
				msg, err := socketPair.Recv()
				if err != nil {
					log.Printf("failed to receive a message from pair socket: %v", err)
					return
				}
				request := pair.RequestPairType(msg[0])
				switch request {
				case pair.RequestNodeListT:
					nodes, err := ns.Nodes()
					if err != nil {
						log.Printf("failed to receive list of nodes from storage, %v", err)
					}

					var nodeList pair.NodeListResponse
					for _, node := range nodes {
						nodeList.Urls = append(nodeList.Urls, node.URL)
					}
					response, err := json.Marshal(nodeList)
					if err != nil {
						log.Printf("failed to marshal list of nodes to json, %v", err)
					}
					err = socketPair.Send(response)
					if err != nil {
						log.Printf("failed to receive a response from pair socket, %v", err)
					}
				case pair.RequestInsertNewNodeT:
					url := msg[1:]
					err := ns.InsertIfNew(string(url))
					if err != nil {
						log.Printf("failed to insert a new node to storage, %v", err)
					}

				case pair.RequestDeleteNodeT:
					url := msg[1:]
					err := ns.Delete(string(url))
					if err != nil {
						log.Printf("failed to delete a node from storage, %v", err)
					}
				default:

				}

			}
		}
	}()

	<-ctx.Done()
	a.Shutdown()
	log.Println("Terminated")
	return nil
}
