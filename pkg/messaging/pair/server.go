package pair

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"time"

	"nodemon/pkg/entities"
	"nodemon/pkg/storing/events"
	"nodemon/pkg/storing/nodes"
	"nodemon/pkg/storing/specific"
	"nodemon/pkg/tools/logging/attrs"

	"github.com/nats-io/nats.go"
	"github.com/pkg/errors"
)

const okMessage = "ok"

func StartPairMessagingServer(
	ctx context.Context,
	natsPairURL string,
	ns nodes.Storage,
	es *events.Storage,
	pew specific.PrivateNodesEventsWriter,
	logger *slog.Logger,
	botRequestsTopic string,
) error {
	nc, err := nats.Connect(natsPairURL)
	if err != nil {
		logger.Error("Failed to connect to nats server", attrs.Error(err))
		return err
	}
	defer nc.Close()

	if len(natsPairURL) == 0 {
		return errors.New("invalid nats URL for pair messaging")
	}

	_, subErr := nc.Subscribe(botRequestsTopic, func(request *nats.Msg) {
		response, handleErr := handleMessage(request.Data, ns, logger, es, pew)
		if handleErr != nil {
			logger.Error("Failed to handle bot request", attrs.Error(handleErr))
			return
		}
		respondErr := request.Respond(response)
		if respondErr != nil {
			logger.Error("Failed to respond to bot request", attrs.Error(respondErr))
			return
		}
	})
	if subErr != nil {
		return subErr
	}
	<-ctx.Done()
	return nil
}

func handleMessage(
	rawMsg []byte,
	ns nodes.Storage,
	logger *slog.Logger,
	es *events.Storage,
	pew specific.PrivateNodesEventsWriter,
) ([]byte, error) {
	if len(rawMsg) == 0 {
		logger.Warn("Empty raw message received from pair socket")
		return nil, nil
	}
	var (
		t   = RequestPairType(rawMsg[0])
		msg = rawMsg[1:] // cut first byte, which is request type
	)
	switch t {
	case RequestNodeListType:
		response, err := handleNodesRequest(ns, false, logger)
		if err != nil {
			return nil, err
		}
		return response, nil
	case RequestSpecificNodeListType:
		response, err := handleNodesRequest(ns, true, logger)
		if err != nil {
			return nil, err
		}
		return response, nil
	case RequestInsertNewNodeType:
		insertRegularNodeIfNew(msg, ns, logger)
	case RequestInsertSpecificNewNodeType:
		insertSpecificNodeIfNew(msg, ns, pew, logger)
	case RequestUpdateNodeType:
		handleUpdateNodeRequest(msg, logger, ns)
	case RequestDeleteNodeType:
		handleDeleteNodeRequest(msg, ns, logger)
	case RequestNodesStatusType, RequestNodeStatementType:
		response := handleNodesStatementsRequest(msg, es, logger)
		return response, nil
	default:
		logger.Error("Unknown request type", slog.Int("type", int(t)), attrs.Binary("message", msg))
	}
	// nats considers a message delivered only if there was a not nil response
	return []byte(okMessage), nil
}

func insertNodeIfNew(url string, ns nodes.Storage, specific bool, logger *slog.Logger) bool {
	appended, err := ns.InsertIfNew(url, specific)
	if err != nil {
		logger.Error("Failed to insert a new node to storage",
			attrs.Error(err), slog.String("node", url), slog.Bool("specific", specific),
		)
	}
	return appended
}

func insertRegularNodeIfNew(msg []byte, ns nodes.Storage, logger *slog.Logger) {
	url := string(msg)
	_ = insertNodeIfNew(url, ns, false, logger)
}

func insertSpecificNodeIfNew(msg []byte, ns nodes.Storage, pew specific.PrivateNodesEventsWriter, logger *slog.Logger) {
	url := string(msg)
	appended := insertNodeIfNew(url, ns, true, logger)
	if appended { // its new specific node
		ts := time.Now().Unix()
		pew.WriteInitialStateForSpecificNode(url, ts) // write unreachable event for the initial specific node state
	}
}

func handleDeleteNodeRequest(msg []byte, ns nodes.Storage, logger *slog.Logger) {
	url := msg
	err := ns.Delete(string(url))
	if err != nil {
		logger.Error("Failed to delete a node from storage", attrs.Error(err))
	}
}

func handleUpdateNodeRequest(msg []byte, logger *slog.Logger, ns nodes.Storage) {
	node := entities.Node{}
	err := json.Unmarshal(msg, &node)
	if err != nil {
		logger.Error("Failed to update a specific node", attrs.Error(err))
	}
	err = ns.Update(node)
	if err != nil {
		logger.Error("Failed to insert a new specific node to storage", attrs.Error(err))
	}
}

func handleNodesRequest(ns nodes.Storage, specific bool, logger *slog.Logger) ([]byte, error) {
	nodesList, err := ns.Nodes(specific)
	if err != nil {
		logger.Error("Failed to get list of nodes from storage",
			attrs.Error(err), slog.Bool("specific", specific),
		)
		return nil, err
	}
	response := NodesListResponse{Nodes: nodesList}
	marshaledResponse, err := json.Marshal(response)
	if err != nil {
		logger.Error("Failed to marshal node list to json", attrs.Error(err))
		return nil, errors.Wrapf(err, "Failed to marshal node list to json")
	}
	return marshaledResponse, nil
}

func handleNodesStatementsRequest(msg []byte, es *events.Storage, logger *slog.Logger) []byte {
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
			logger.Error("Failed to find all statehashes by last height", attrs.Error(err))
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
		logger.Error("Failed to marshal node status to json", attrs.Error(err))
	}
	return response
}
