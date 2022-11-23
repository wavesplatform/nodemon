package pair

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"

	"github.com/pkg/errors"
	"go.nanomsg.org/mangos/v3/protocol"
	pairProtocol "go.nanomsg.org/mangos/v3/protocol/pair"
	"go.uber.org/zap"
	"nodemon/pkg/entities"
	"nodemon/pkg/messaging/pair"
)

func StartPairMessagingClient(ctx context.Context, nanomsgURL string, requestPair chan pair.RequestPair, responsePair chan pair.ResponsePair, logger *zap.Logger) error {
	pairSocket, err := pairProtocol.NewSocket()
	if err != nil {
		return errors.Wrap(err, "failed to get new pair socket")
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

				switch r := request.(type) {
				case *pair.NodesListRequest:
					if r.Specific {
						message.WriteByte(byte(pair.RequestSpecificNodeListT))

					} else {
						message.WriteByte(byte(pair.RequestNodeListT))
					}

					err = pairSocket.Send(message.Bytes())
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
				case *pair.InsertNewNodeRequest:
					if r.Specific {
						message.WriteByte(byte(pair.RequestInsertSpecificNewNodeT))
					} else {
						message.WriteByte(byte(pair.RequestInsertNewNodeT))
					}

					message.Write([]byte(r.Url))
					err = pairSocket.Send(message.Bytes())
					if err != nil {
						logger.Error("failed to send message", zap.Error(err))
					}

				case *pair.DeleteNodeRequest:
					message.WriteByte(byte(pair.RequestDeleteNodeT))

					message.Write([]byte(r.Url))
					err = pairSocket.Send(message.Bytes())
					if err != nil {
						logger.Error("failed to send a request to pair socket", zap.Error(err))
					}
				case *pair.NodesStatusRequest:
					message.WriteByte(byte(pair.RequestNodesStatus))

					message.Write([]byte(strings.Join(r.Urls, ",")))
					err = pairSocket.Send(message.Bytes())
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
				case *pair.NodeStatementRequest:
					message.WriteByte(byte(pair.RequestNodeStatement))

					req, err := json.Marshal(entities.NodeHeight{URL: r.Url, Height: r.Height})
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
