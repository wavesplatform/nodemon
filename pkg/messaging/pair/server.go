package pair

import (
	"github.com/pkg/errors"
	"go.nanomsg.org/mangos/v3/protocol"
	"go.nanomsg.org/mangos/v3/protocol/pair"
	"log"
	"strings"
)

func StartPairMessagingServer(nanomsgURL string) (protocol.Socket, error) {
	if len(nanomsgURL) == 0 || len(strings.Fields(nanomsgURL)) > 1 {
		log.Printf("Invalid nanomsg IPC URL '%s'", nanomsgURL)
		return nil, errors.New("invalid nanomsg IPC URL")
	}
	socket, err := pair.NewSocket()
	if err != nil {
		log.Printf("Failed to get new pair socket: %v", err)
		return nil, err
	}
	if err := socket.Listen(nanomsgURL); err != nil {
		log.Printf("Failed to listen on pair socket: %v", err)
		return nil, err
	}

	return socket, nil
}
