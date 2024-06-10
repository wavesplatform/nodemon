package pair

import (
	"context"
	"encoding/json"
	"strings"

	"nodemon/pkg/entities"
	"nodemon/pkg/storing/events"
	"nodemon/pkg/storing/nodes"

	"github.com/pkg/errors"
	"go.nanomsg.org/mangos/v3/protocol"
	"go.nanomsg.org/mangos/v3/protocol/pair"
	"go.uber.org/zap"
)

func StartPairMessagingServer(
	ctx context.Context,
	nanomsgURL string,
	ns nodes.Storage,
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

	loopErr := enterLoop(ctx, socket, logger, ns, es)
	if loopErr != nil && !errors.Is(loopErr, context.Canceled) {
		return loopErr
	}
	return nil
}

func enterLoop(
	ctx context.Context,
	socket protocol.Socket,
	logger *zap.Logger,
	ns nodes.Storage,
	es *events.Storage,
) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
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
	ns nodes.Storage,
	logger *zap.Logger,
	socket protocol.Socket,
	es *events.Storage,
) error {
	if len(rawMsg) == 0 {
		logger.Warn("empty raw message received from pair socket")
		return nil
	}
	var (
		t   = RequestPairType(rawMsg[0])
		msg = rawMsg[1:] // cut first byte, which is request type
	)
	switch t {
	case RequestNodeListType:
		if err := handleNodesRequest(ns, false, logger, socket); err != nil {
			return err
		}
	case RequestSpecificNodeListType:
		if err := handleNodesRequest(ns, true, logger, socket); err != nil {
			return err
		}
	case RequestInsertNewNodeType:
		insertNodeIfNew(msg, ns, false, logger)
	case RequestInsertSpecificNewNodeType:
		insertNodeIfNew(msg, ns, true, logger)
	case RequestUpdateNodeType:
		handleUpdateNodeRequest(msg, logger, ns)
	case RequestDeleteNodeType:
		handleDeleteNodeRequest(msg, ns, logger)
	case RequestNodesStatusType, RequestNodeStatementType:
		handleNodesStatementsRequest(msg, es, logger, socket)
	default:
		logger.Error("Unknown request type", zap.Int("type", int(t)), zap.Binary("message", msg))
	}
	return nil
}

func insertNodeIfNew(msg []byte, ns nodes.Storage, specific bool, logger *zap.Logger) {
	url := msg
	err := ns.InsertIfNew(string(url), specific)
	if err != nil {
		logger.Error("Failed to insert a new node to storage",
			zap.Error(err), zap.Bool("specific", specific),
		)
	}
}

func handleDeleteNodeRequest(msg []byte, ns nodes.Storage, logger *zap.Logger) {
	url := msg
	err := ns.Delete(string(url))
	if err != nil {
		logger.Error("Failed to delete a node from storage", zap.Error(err))
	}
}

func handleUpdateNodeRequest(msg []byte, logger *zap.Logger, ns nodes.Storage) {
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

func handleNodesRequest(ns nodes.Storage, specific bool, logger *zap.Logger, socketPair protocol.Socket) error {
	nodesList, err := ns.Nodes(specific)
	if err != nil {
		logger.Error("Failed to get list of nodes from storage",
			zap.Error(err), zap.Bool("specific", specific),
		)
		return err
	}
	response := NodesListResponse{Nodes: nodesList}
	marshaledResponse, err := json.Marshal(response)
	if err != nil {
		logger.Error("Failed to marshal node list to json", zap.Error(err))
	}
	err = socketPair.Send(marshaledResponse)
	if err != nil {
		logger.Error("Failed to send a node list to pair socket", zap.Error(err))
	}
	return nil
}

func handleNodesStatementsRequest(msg []byte, es *events.Storage, logger *zap.Logger, socketPair protocol.Socket) {
	listOfNodes := strings.Split(string(msg), ",")
	var nodesStatusResp NodesStatementsResponse

	statements, err := es.FindAllStatementsOnCommonHeight(listOfNodes)
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
			BlockID:   statement.BlockID,
			Generator: statement.Generator,
		}
		nodesStatusResp.NodesStatements = append(nodesStatusResp.NodesStatements, nodeStat)
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
