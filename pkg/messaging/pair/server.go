package pair

import (
	"context"
	"encoding/json"
	"strings"

	"nodemon/pkg/entities"
	"nodemon/pkg/storing/events"
	nodesStor "nodemon/pkg/storing/nodes"

	"github.com/pkg/errors"
	"go.nanomsg.org/mangos/v3/protocol"
	"go.nanomsg.org/mangos/v3/protocol/pair"
	"go.uber.org/zap"
)

func StartPairMessagingServer(
	ctx context.Context,
	nanomsgURL string,
	ns nodesStor.Storage,
	es *events.Storage,
	logger *zap.Logger,
) error {
	if len(nanomsgURL) == 0 || len(strings.Fields(nanomsgURL)) > 1 {
		return errors.New("invalid nanomsg IPC URL for pair socket")
	}
	socket, sockErr := pair.NewSocket()
	if sockErr != nil {
		return sockErr
	}
	defer func(socketPair protocol.Socket) {
		if err := socketPair.Close(); err != nil {
			logger.Error("Failed to close pair socket", zap.Error(err))
		}
	}(socket)

	if err := socket.Listen(nanomsgURL); err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			rawMsg, recvErr := socket.Recv()
			if recvErr != nil {
				logger.Error("Failed to receive a message from pair socket", zap.Error(recvErr))
				return recvErr
			}
			err := handleMessage(rawMsg, ns, logger, socket, es)
			if err != nil {
				return err
			}
		}
	}
}

func handleMessage(
	rawMsg []byte,
	ns nodesStor.Storage,
	logger *zap.Logger,
	socket protocol.Socket,
	es *events.Storage,
) error {
	if len(rawMsg) == 0 {
		return errors.New("empty message")
	}
	var (
		request = RequestPairType(rawMsg[0])
		msg     = rawMsg[1:] // cut first byte
	)
	switch request {
	case RequestNodeListType:
		nodes, err := ns.Nodes(false)
		if err != nil {
			logger.Error("Failed to get list of nodes from storage", zap.Error(err))
			return err
		}
		handleNodesRequest(nodes, logger, socket)
	case RequestSpecificNodeListType:
		nodes, err := ns.Nodes(true)
		if err != nil {
			logger.Error("Failed to receive list of specific nodes from storage", zap.Error(err))
			return err
		}
		handleNodesRequest(nodes, logger, socket)
	case RequestInsertNewNodeType:
		url := msg
		err := ns.InsertIfNew(string(url), false)
		if err != nil {
			logger.Error("Failed to insert a new node to storage", zap.Error(err))
		}
	case RequestInsertSpecificNewNodeType:
		url := msg
		err := ns.InsertIfNew(string(url), true)
		if err != nil {
			logger.Error("Failed to insert a new specific node to storage", zap.Error(err))
		}
	case RequestUpdateNodeType:
		handleUpdateNodeRequest(msg, logger, ns)
	case RequestDeleteNodeType:
		handleDeleteNodeRequest(msg, ns, logger)
	case RequestNodesStatusType:
		handleNodeStatusRequest(msg, es, logger, socket)
	case RequestNodeStatementType:
		handleNodeStatementRequest(msg, logger, es, socket)
	default:
		logger.Error("Unknown request type", zap.String("request", string(request)))
	}
	return nil
}

func handleDeleteNodeRequest(msg []byte, ns nodesStor.Storage, logger *zap.Logger) {
	url := msg
	err := ns.Delete(string(url))
	if err != nil {
		logger.Error("Failed to delete a node from storage", zap.Error(err))
	}
}

func handleUpdateNodeRequest(msg []byte, logger *zap.Logger, ns nodesStor.Storage) {
	node := entities.Node{}
	err := json.Unmarshal(msg, &node)
	if err != nil {
		logger.Error("Failed to update a specific node", zap.Error(err))
	}
	err = ns.Update(node)
	if err != nil {
		logger.Error("Failed to insert a new specific node to storage", zap.Error(err))
	}
}

func handleNodesRequest(nodes []entities.Node, logger *zap.Logger, socketPair protocol.Socket) {
	nodeList := NodesListResponse{Nodes: nodes}
	response, err := json.Marshal(nodeList)
	if err != nil {
		logger.Error("Failed to marshal node list to json", zap.Error(err))
	}
	err = socketPair.Send(response)
	if err != nil {
		logger.Error("Failed to send a node list to pair socket", zap.Error(err))
	}
}

func handleNodeStatementRequest(msg []byte, logger *zap.Logger, es *events.Storage, socketPair protocol.Socket) {
	nodeHeight := entities.NodeHeight{}
	err := json.Unmarshal(msg, &nodeHeight)
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
}

func handleNodeStatusRequest(msg []byte, es *events.Storage, logger *zap.Logger, socketPair protocol.Socket) {
	listOfNodes := msg
	nodes := strings.Split(string(listOfNodes), ",")
	var nodesStatusResp NodesStatusResponse

	statements, err := es.FindAllStateHashesOnCommonHeight(nodes)
	switch {
	case errors.Is(err, events.ErrBigHeightDifference):
		nodesStatusResp.ErrMessage = events.ErrBigHeightDifference.Error()
	case errors.Is(err, events.ErrStorageIsNotReady):
		nodesStatusResp.ErrMessage = events.ErrStorageIsNotReady.Error()
	default:
		if err != nil {
			logger.Error("failed to find all statehashes by last height", zap.Error(err))
		}
	}

	for _, statement := range statements {
		nodeStat := NodeStatement{
			Height:    statement.Height,
			StateHash: statement.StateHash,
			URL:       statement.Node,
			Status:    statement.Status,
		}
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
}
