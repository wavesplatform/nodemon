package pair

import (
	"context"
	"encoding/json"
	"log"
	"strings"

	"github.com/pkg/errors"
	"go.nanomsg.org/mangos/v3/protocol"
	"go.nanomsg.org/mangos/v3/protocol/pair"
	"nodemon/pkg/storing/nodes"
)

func StartPairMessagingServer(ctx context.Context, nanomsgURL string, ns *nodes.Storage) (error) {
	if len(nanomsgURL) == 0 || len(strings.Fields(nanomsgURL)) > 1 {
		log.Printf("Invalid nanomsg IPC URL for pair socket'%s'", nanomsgURL)
		return errors.New("invalid nanomsg IPC URL for pair socket")
	}
	socketPair, err := pair.NewSocket()
	if err != nil {
		log.Printf("Failed to get new pair socket: %v", err)
		return err
	}
	defer func(socketPair protocol.Socket) {
		if err := socketPair.Close(); err != nil {
			log.Printf("Failed to close pubsub socket: %v", err)
		}
	}(socketPair)

	if err := socketPair.Listen(nanomsgURL); err != nil {
		log.Printf("Failed to listen on pair socket: %v", err)
		return err
	}

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
				request := RequestPairType(msg[0])
				switch request {
				case RequestNodeListT:
					nodes, err := ns.Nodes()
					if err != nil {
						log.Printf("failed to receive list of nodes from storage, %v", err)
					}

					var nodeList NodeListResponse
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
				case RequestInsertNewNodeT:
					url := msg[1:]
					err := ns.InsertIfNew(string(url))
					if err != nil {
						log.Printf("failed to insert a new node to storage, %v", err)
					}

				case RequestDeleteNodeT:
					url := msg[1:]
					err := ns.Delete(string(url))
					if err != nil {
						log.Printf("failed to delete a node from storage, %v", err)
					}
				default:
					log.Printf("request type to the pair socket is unknown: %c", request)
				}

			}
		}
	}()

	return nil
}
