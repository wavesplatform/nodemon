package pair

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/pkg/errors"
	"go.nanomsg.org/mangos/v3/protocol"
	"go.nanomsg.org/mangos/v3/protocol/pair"
	"go.uber.org/zap"
	"nodemon/pkg/entities"
	"nodemon/pkg/storing/events"
	"nodemon/pkg/storing/nodes"
)

func StartPairMessagingServer(ctx context.Context, nanomsgURL string, ns *nodes.Storage, es *events.Storage, logger *zap.Logger) error {
	if len(nanomsgURL) == 0 || len(strings.Fields(nanomsgURL)) > 1 {
		return errors.New("invalid nanomsg IPC URL for pair socket")
	}
	socketPair, err := pair.NewSocket()
	if err != nil {
		return err
	}
	defer func(socketPair protocol.Socket) {
		if err := socketPair.Close(); err != nil {
			logger.Error("Failed to close pair socket", zap.Error(err))
		}
	}(socketPair)

	if err := socketPair.Listen(nanomsgURL); err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			msg, err := socketPair.Recv()
			if err != nil {
				logger.Error("Failed to receive a message from pair socket", zap.Error(err))
				return nil
			}
			request := RequestPairType(msg[0])
			switch request {
			case RequestNodeListT, RequestSpecificNodeListT:
				var nodes []entities.Node
				if request == RequestNodeListT {
					nodes, err = ns.Nodes(false)
					if err != nil {
						logger.Error("Failed to get list of nodes from storage", zap.Error(err))
						return err
					}
				} else {
					nodes, err = ns.Nodes(true)
					if err != nil {
						logger.Error("Failed to receive list of specific nodes from storage", zap.Error(err))
						return err
					}
				}
				nodeList := NodesListResponse{Nodes: nodes}
				response, err := json.Marshal(nodeList)
				if err != nil {
					logger.Error("Failed to marshal node list to json", zap.Error(err))
				}
				err = socketPair.Send(response)
				if err != nil {
					logger.Error("Failed to send a node list to pair socket", zap.Error(err))
				}

			case RequestInsertNewNodeT:
				url := msg[1:]
				err := ns.InsertIfNew(string(url), false)
				if err != nil {
					logger.Error("Failed to insert a new node to storage", zap.Error(err))
				}
			case RequestInsertSpecificNewNodeT:
				url := msg[1:]
				err := ns.InsertIfNew(string(url), true)
				if err != nil {
					logger.Error("Failed to insert a new specific node to storage", zap.Error(err))
				}
			case RequestUpdateNode:
				node := entities.Node{}
				err := json.Unmarshal(msg[1:], &node)
				if err != nil {
					logger.Error("Failed to update a specific node", zap.Error(err))
				}
				err = ns.Update(node)
				if err != nil {
					logger.Error("Failed to insert a new specific node to storage", zap.Error(err))
				}

			case RequestDeleteNodeT:
				url := msg[1:]
				err := ns.Delete(string(url))
				if err != nil {
					logger.Error("Failed to delete a node from storage", zap.Error(err))
				}
			case RequestNodesStatus:
				listOfNodes := msg[1:]
				nodes := strings.Split(string(listOfNodes), ",")
				var nodesStatusResp NodesStatusResponse

				statements, err := es.FindAllStateHashesOnCommonHeight(nodes)
				switch {
				case errors.Is(err, events.BigHeightDifference):
					nodesStatusResp.ErrMessage = events.BigHeightDifference.Error()
				case errors.Is(err, events.StorageIsNotReady):
					nodesStatusResp.ErrMessage = events.StorageIsNotReady.Error()
				default:
					if err != nil {
						logger.Error("failed to find all statehashes by last height", zap.Error(err))
					}
				}

				for _, statement := range statements {
					nodeStat := NodeStatement{Height: statement.Height, StateHash: statement.StateHash, Url: statement.Node, Status: statement.Status}
					nodesStatusResp.NodesStatus = append(nodesStatusResp.NodesStatus, nodeStat)
				}
				response, err := json.Marshal(nodesStatusResp)
				if err != nil {
					logger.Error("Failed to marshal node status to json", zap.Error(err))
				}
				err = socketPair.Send(response)
				if err != nil {
					logger.Error("Failed to send a response from pair socket", zap.Error(err))
				}
			case RequestNodeStatement:
				nodeHeight := entities.NodeHeight{}
				err := json.Unmarshal(msg[1:], &nodeHeight)
				if err != nil {
					logger.Error("Failed to unmarshal node height from json", zap.Error(err))
				}
				var nodeStatementResp NodeStatementResponse

				statement, err := es.GetFullStatementAtHeight(nodeHeight.URL, nodeHeight.Height)
				if err != nil {
					nodeStatementResp.ErrMessage = err.Error()
				}
				nodeStatementResp.NodeStatement = statement
				response, err := json.Marshal(nodeStatementResp)
				if err != nil {
					logger.Error("Failed to marshal node status to json", zap.Error(err))
				}
				err = socketPair.Send(response)
				if err != nil {
					logger.Error("Failed to send a response from pair socket", zap.Error(err))
				}
			default:
				logger.Error("Unknown request type", zap.String("request", string(request)))
			}

		}
	}
}
