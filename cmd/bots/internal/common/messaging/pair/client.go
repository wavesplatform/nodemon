package pair

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"

	"nodemon/pkg/entities"
	"nodemon/pkg/messaging/pair"

	"github.com/pkg/errors"
	"go.nanomsg.org/mangos/v3/protocol"
	pairProtocol "go.nanomsg.org/mangos/v3/protocol/pair"
	"go.uber.org/zap"
)

func StartPairMessagingClient(
	ctx context.Context,
	nanomsgURL string,
	requestPair <-chan pair.Request,
	responsePair chan<- pair.Response,
	logger *zap.Logger,
) error {
	pairSocket, sockErr := pairProtocol.NewSocket()
	if sockErr != nil {
		return errors.Wrap(sockErr, "failed to get new pair socket")
	}

	defer func(pairSocket protocol.Socket) {
		if err := pairSocket.Close(); err != nil {
			logger.Error("failed to close a pair socket", zap.Error(err))
		}
	}(pairSocket)

	if err := pairSocket.Dial(nanomsgURL); err != nil {
		return errors.Wrap(err, "failed to dial on pair socket")
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				request := <-requestPair

				message := &bytes.Buffer{}
				message.WriteByte(byte(request.RequestType()))

				switch r := request.(type) {
				case *pair.NodesListRequest:
					handleNodesListRequest(pairSocket, message, logger, responsePair)
				case *pair.InsertNewNodeRequest:
					handleInsertNewNodeRequest(r.URL, message, pairSocket, logger)
				case *pair.UpdateNodeRequest:
					handleUpdateNodeRequest(r.URL, r.Alias, logger, message, pairSocket)
				case *pair.DeleteNodeRequest:
					message.WriteString(r.URL)
					err := pairSocket.Send(message.Bytes())
					if err != nil {
						logger.Error("failed to send a request to pair socket", zap.Error(err))
					}
				case *pair.NodesStatusRequest:
					handleNodesStatusRequest(r.URLs, message, pairSocket, logger, responsePair)
				case *pair.NodeStatementRequest:
					handleNodesStatementRequest(r.URL, r.Height, logger, message, pairSocket, responsePair)
				default:
					logger.Error("unknown request type to pair socket")
				}
			}
		}
	}()

	<-ctx.Done()
	logger.Info("pair messaging service finished")
	return nil
}

func handleNodesStatementRequest(
	url string,
	height int,
	logger *zap.Logger,
	message *bytes.Buffer,
	pairSocket protocol.Socket,
	responsePair chan<- pair.Response,
) {
	req, err := json.Marshal(entities.NodeHeight{URL: url, Height: height})
	if err != nil {
		logger.Error("failed to marshal message to pair socket", zap.Error(err))
	}

	message.Write(req)
	err = pairSocket.Send(message.Bytes())
	if err != nil {
		logger.Error("failed to send a request to pair socket", zap.Error(err))
	}

	response, err := pairSocket.Recv()
	if err != nil {
		logger.Error("failed to receive message from pair socket", zap.Error(err))
	}
	nodeStatementResp := pair.NodeStatementResponse{}
	err = json.Unmarshal(response, &nodeStatementResp)
	if err != nil {
		logger.Error("failed to unmarshal message from pair socket", zap.Error(err))
	}
	responsePair <- &nodeStatementResp
}

func handleNodesStatusRequest(
	urls []string,
	message *bytes.Buffer,
	pairSocket protocol.Socket,
	logger *zap.Logger,
	responsePair chan<- pair.Response,
) {
	message.WriteString(strings.Join(urls, ","))
	err := pairSocket.Send(message.Bytes())
	if err != nil {
		logger.Error("failed to send a request to pair socket", zap.Error(err))
	}

	response, err := pairSocket.Recv()
	if err != nil {
		logger.Error("failed to receive message from pair socket", zap.Error(err))
	}
	nodesStatusResp := pair.NodesStatusResponse{}
	err = json.Unmarshal(response, &nodesStatusResp)
	if err != nil {
		logger.Error("failed to unmarshal message from pair socket", zap.Error(err))
	}
	responsePair <- &nodesStatusResp
}

func handleUpdateNodeRequest(url, alias string, logger *zap.Logger, message *bytes.Buffer, pairSocket protocol.Socket) {
	node := entities.Node{URL: url, Enabled: true, Alias: alias}
	nodeInfo, err := json.Marshal(node)
	if err != nil {
		logger.Error("failed to marshal node's info")
	}
	message.Write(nodeInfo)
	err = pairSocket.Send(message.Bytes())
	if err != nil {
		logger.Error("failed to send message", zap.Error(err))
	}
}

func handleInsertNewNodeRequest(url string, message *bytes.Buffer, pairSocket protocol.Socket, logger *zap.Logger) {
	message.WriteString(url)
	err := pairSocket.Send(message.Bytes())
	if err != nil {
		logger.Error("failed to send message", zap.Error(err))
	}
}

func handleNodesListRequest(
	pairSocket protocol.Socket,
	message *bytes.Buffer,
	logger *zap.Logger,
	responsePair chan<- pair.Response,
) {
	err := pairSocket.Send(message.Bytes())
	if err != nil {
		logger.Error("failed to send message", zap.Error(err))
	}

	response, err := pairSocket.Recv()
	if err != nil {
		logger.Error("failed to receive message", zap.Error(err))
	}
	nodeList := pair.NodesListResponse{}
	err = json.Unmarshal(response, &nodeList)
	if err != nil {
		logger.Error("failed to unmarshal message", zap.Error(err))
	}
	responsePair <- &nodeList
}
