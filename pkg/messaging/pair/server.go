package pair

import (
	"context"
	"encoding/json"
	"log"
	"strings"

	"github.com/pkg/errors"
	"go.nanomsg.org/mangos/v3/protocol"
	"go.nanomsg.org/mangos/v3/protocol/pair"
	"nodemon/pkg/entities"
	"nodemon/pkg/storing/events"
	"nodemon/pkg/storing/nodes"
)

func StartPairMessagingServer(ctx context.Context, nanomsgURL string, ns *nodes.Storage, es *events.Storage) error {
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

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			msg, err := socketPair.Recv()
			if err != nil {
				log.Printf("failed to receive a message from pair socket: %v", err)
				return nil
			}
			request := RequestPairType(msg[0])
			switch request {
			case RequestNodeListT, RequestSpecificNodeListT:
				var nodes []entities.Node
				if request == RequestNodeListT {
					nodes, err = ns.Nodes(false)
					if err != nil {
						log.Printf("failed to receive list of nodes from storage, %v", err)
						return err
					}
				} else {
					nodes, err = ns.Nodes(true)
					if err != nil {
						log.Printf("failed to receive list of specific nodes from storage, %v", err)
						return err
					}
				}
				var nodeList NodesListResponse

				nodeList.Urls = make([]string, len(nodes))
				for i, node := range nodes {
					nodeList.Urls[i] = node.URL
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
			case RequestInsertSpecificNewNodeT:
				url := msg[1:]
				err := ns.InsertSpecificIfNew(string(url))
				if err != nil {
					log.Printf("failed to insert a new specific node to storage, %v", err)
				}
			case RequestDeleteNodeT:
				url := msg[1:]
				err := ns.Delete(string(url))
				if err != nil {
					log.Printf("failed to delete a node from storage, %v", err)
				}
			case RequestNodesStatus:
				listOfNodes := msg[1:]
				nodes := strings.Split(string(listOfNodes), ",")
				var nodesStatusResp NodesStatusResponse
				statements, err := es.FindAllStatehashesOnCommonHeight(nodes)
				switch {
				case errors.Is(err, events.BigHeightDifference):
					nodesStatusResp.Err = events.BigHeightDifference.Error()
				case errors.Is(err, events.StorageIsNotReady):
					nodesStatusResp.Err = events.StorageIsNotReady.Error()
				default:
					log.Printf("failed to find all statehashes by last height")
				}
				for _, statement := range statements {
					nodeStat := NodeStatement{Height: statement.Height, StateHash: statement.StateHash, Url: statement.Node, Status: statement.Status}
					nodesStatusResp.NodesStatus = append(nodesStatusResp.NodesStatus, nodeStat)
				}
				response, err := json.Marshal(nodesStatusResp)
				if err != nil {
					log.Printf("failed to marshal list of nodes to json, %v", err)
				}
				err = socketPair.Send(response)
				if err != nil {
					log.Printf("failed to receive a response from pair socket, %v", err)
				}

			default:
				log.Printf("request type to the pair socket is unknown: %c", request)
			}

		}
	}
}
