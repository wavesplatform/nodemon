package pubsub

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"strings"

	"github.com/pkg/errors"
	"go.nanomsg.org/mangos/v3/protocol"
	"nodemon/pkg/entities"
	"nodemon/pkg/messaging"

	"go.nanomsg.org/mangos/v3/protocol/pub"
	_ "go.nanomsg.org/mangos/v3/transport/all"
)

func StartPubMessagingServer(ctx context.Context, nanomsgURL string, alerts <-chan entities.Alert) error {
	if len(nanomsgURL) == 0 || len(strings.Fields(nanomsgURL)) > 1 {
		log.Printf("Invalid nanomsg IPC URL for pub server'%s'", nanomsgURL)
		return errors.New("invalid nanomsg IPC URL for pub sub socket")
	}

	socketPub, err := pub.NewSocket()
	if err != nil {
		log.Printf("Failed to get new pub socket: %v", err)
		return err
	}
	defer func(socketPubSub protocol.Socket) {
		if err := socketPubSub.Close(); err != nil {
			log.Printf("Failed to close pub socket: %v", err)
		}
	}(socketPub)

	if err := socketPub.Listen(nanomsgURL); err != nil {
		log.Printf("Failed to listen on pub socket: %v", err)
		return err
	}

	// pubsub messaging

	for {
		select {
		case <-ctx.Done():
			return nil
		case alert := <-alerts:
			log.Printf("Alert has been generated: %v", alert)

			jsonAlert, err := json.Marshal(
				messaging.Alert{
					AlertDescription: alert.ShortDescription(),
					Level:            alert.Level(),
					Details:          alert.Message(),
				})
			if err != nil {
				log.Printf("failed to marshal alert to json, %v", err)
			}

			message := &bytes.Buffer{}
			message.WriteByte(byte(alert.Type()))
			message.Write(jsonAlert)
			err = socketPub.Send(message.Bytes())
			if err != nil {
				log.Printf("failed to send a message to socket, %v", err)
			}
		}
	}
}
