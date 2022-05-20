package pubsub

import (
	"github.com/pkg/errors"
	"log"
	"strings"

	"go.nanomsg.org/mangos/v3/protocol"
	"go.nanomsg.org/mangos/v3/protocol/pub"
	_ "go.nanomsg.org/mangos/v3/transport/all"
)

func StartPubSubMessagingServer(nanomsgURL string) (protocol.Socket, error) {
	if len(nanomsgURL) == 0 || len(strings.Fields(nanomsgURL)) > 1 {
		log.Printf("Invalid nanomsg IPC URL for pubsub server'%s'", nanomsgURL)
		return nil, errors.New("invalid nanomsg IPC URL")
	}

	socket, err := pub.NewSocket()
	if err != nil {
		log.Printf("Failed to get new pub socket: %v", err)
		return nil, err
	}
	if err := socket.Listen(nanomsgURL); err != nil {
		log.Printf("Failed to listen on pub socket: %v", err)
		return nil, err
	}

	return socket, nil
}
