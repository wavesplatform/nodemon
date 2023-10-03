package messaging

import (
	"bytes"
	"context"
	"encoding/json"
	stderr "errors"
	"fmt"
	"strings"
	"time"

	"nodemon/pkg/entities"
	"nodemon/pkg/messaging/pair"

	"github.com/pkg/errors"
	"go.nanomsg.org/mangos/v3/protocol"
	pairProtocol "go.nanomsg.org/mangos/v3/protocol/pair"
	"go.uber.org/zap"
)

const (
	defaultSendTimeout = 5 * time.Second
	defaultRecvTimeout = 5 * time.Second
)

const defaultResponseTimeout = 5 * time.Second

type sendRecvDeadlineSocketWrapper struct {
	protocol.Socket
}

func (w sendRecvDeadlineSocketWrapper) Send(data []byte) (err error) {
	defer func() { // reset deadline in any case
		setOptErr := w.Socket.SetOption(protocol.OptionSendDeadline, time.Duration(0))
		if setOptErr != nil {
			if err != nil {
				err = stderr.Join(err, setOptErr)
			} else {
				err = setOptErr
			}
		}
	}()
	// set deadline for current send
	if setOptErr := w.Socket.SetOption(protocol.OptionSendDeadline, defaultSendTimeout); setOptErr != nil {
		return setOptErr
	}
	return w.Socket.Send(data)
}

func (w sendRecvDeadlineSocketWrapper) Recv() (_ []byte, err error) {
	defer func() { // reset deadline in any case
		setOptErr := w.Socket.SetOption(protocol.OptionRecvDeadline, time.Duration(0))
		if setOptErr != nil {
			if err != nil {
				err = stderr.Join(err, setOptErr)
			} else {
				err = setOptErr
			}
		}
	}()
	// set deadline for current recv
	if setOptErr := w.Socket.SetOption(protocol.OptionRecvDeadline, defaultRecvTimeout); setOptErr != nil {
		return nil, setOptErr
	}
	return w.Socket.Recv()
}

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
	defer func() {
		_ = pairSocket.Close() // can be ignored, only possible error is protocol.ErrClosed
	}()

	if err := pairSocket.Dial(nanomsgURL); err != nil {
		return errors.Wrapf(err, "failed to dial '%s' on pair socket", nanomsgURL)
	}

	done := runPairLoop(ctx, requestPair, sendRecvDeadlineSocketWrapper{pairSocket}, logger, responsePair)

	<-ctx.Done()
	logger.Info("stopping pair messaging service...")
	<-done
	logger.Info("pair messaging service finished")
	return nil
}

func runPairLoop(
	ctx context.Context,
	requestPair <-chan pair.Request,
	pairSocket protocol.Socket,
	logger *zap.Logger,
	responsePair chan<- pair.Response,
) <-chan struct{} {
	ch := make(chan struct{})
	go func(done chan<- struct{}) {
		defer close(done)
		for {
			if ctx.Err() != nil {
				return
			}
			select {
			case <-ctx.Done():
				return
			case request := <-requestPair:
				message := &bytes.Buffer{}
				message.WriteByte(byte(request.RequestType()))

				err := handlePairRequest(ctx, request, pairSocket, message, logger, responsePair)
				if err != nil {
					logger.Error("failed to handle pair request",
						zap.String("request-type", fmt.Sprintf("(%T)", request)),
						zap.Error(err),
					)
				}
			}
		}
	}(ch)
	return ch
}

func handlePairRequest(
	ctx context.Context,
	request pair.Request,
	pairSocket protocol.Socket,
	message *bytes.Buffer,
	logger *zap.Logger,
	responsePair chan<- pair.Response,
) error {
	switch r := request.(type) {
	case *pair.NodesListRequest:
		return handleNodesListRequest(ctx, pairSocket, message, logger, responsePair)
	case *pair.InsertNewNodeRequest:
		return handleInsertNewNodeRequest(r.URL, message, pairSocket)
	case *pair.UpdateNodeRequest:
		return handleUpdateNodeRequest(r.URL, r.Alias, message, pairSocket)
	case *pair.DeleteNodeRequest:
		message.WriteString(r.URL)
		if sendErr := pairSocket.Send(message.Bytes()); sendErr != nil {
			return errors.Wrap(sendErr, "failed handle delete node request and send data to a pair socket")
		}
		return nil
	case *pair.NodesStatusRequest:
		return handleNodesStatusRequest(ctx, r.URLs, message, pairSocket, logger, responsePair)
	case *pair.NodeStatementRequest:
		return handleNodesStatementRequest(ctx, r.URL, r.Height, logger, message, pairSocket, responsePair)
	default:
		return errors.New("unknown request type to pair socket")
	}
}

func handleNodesStatementRequest(
	ctx context.Context,
	url string,
	height int,
	logger *zap.Logger,
	message *bytes.Buffer,
	pairSocket protocol.Socket,
	responsePair chan<- pair.Response,
) error {
	ctx, cancel := context.WithTimeout(ctx, defaultResponseTimeout)
	defer cancel()

	req, err := json.Marshal(entities.NodeHeight{URL: url, Height: height})
	if err != nil {
		return errors.Wrap(err, "failed to marshal message to pair socket")
	}

	message.Write(req)
	err = pairSocket.Send(message.Bytes())
	if err != nil {
		return errors.Wrap(err, "failed to send a request to pair socket")
	}

	response, err := pairSocket.Recv()
	if err != nil {
		return errors.Wrap(err, "failed to receive message from pair socket")
	}
	nodeStatementResp := pair.NodeStatementResponse{}
	err = json.Unmarshal(response, &nodeStatementResp)
	if err != nil {
		return errors.Wrap(err, "failed to unmarshal message from pair socket")
	}
	select {
	case responsePair <- &nodeStatementResp:
		return nil
	case <-ctx.Done():
		logger.Error("failed to send node statement response, timeout exceeded",
			zap.Duration("timeout", defaultResponseTimeout),
			zap.ByteString("node-statement-response", response),
			zap.Error(ctx.Err()),
		)
		return ctx.Err()
	}
}

func handleNodesStatusRequest(
	ctx context.Context,
	urls []string,
	message *bytes.Buffer,
	pairSocket protocol.Socket,
	logger *zap.Logger,
	responsePair chan<- pair.Response,
) error {
	ctx, cancel := context.WithTimeout(ctx, defaultResponseTimeout)
	defer cancel()

	message.WriteString(strings.Join(urls, ","))
	err := pairSocket.Send(message.Bytes())
	if err != nil {
		return errors.Wrap(err, "failed to send a request to pair socket")
	}

	response, err := pairSocket.Recv()
	if err != nil {
		return errors.Wrap(err, "failed to receive message from pair socket")
	}
	nodesStatusResp := pair.NodesStatusResponse{}
	err = json.Unmarshal(response, &nodesStatusResp)
	if err != nil {
		return errors.Wrap(err, "failed to unmarshal message from pair socket")
	}
	select {
	case responsePair <- &nodesStatusResp:
		return nil
	case <-ctx.Done():
		logger.Error("failed to send nodes status response, timeout exceeded",
			zap.Duration("timeout", defaultResponseTimeout),
			zap.ByteString("nodes-status-response", response),
			zap.Error(ctx.Err()),
		)
		return ctx.Err()
	}
}

func handleUpdateNodeRequest(url, alias string, message *bytes.Buffer, pairSocket protocol.Socket) error {
	node := entities.Node{URL: url, Enabled: true, Alias: alias}
	nodeInfo, err := json.Marshal(node)
	if err != nil {
		return errors.Wrap(err, "failed to marshal node's info")
	}
	message.Write(nodeInfo)
	err = pairSocket.Send(message.Bytes())
	if err != nil {
		return errors.Wrap(err, "failed to send message")
	}
	return nil
}

func handleInsertNewNodeRequest(url string, message *bytes.Buffer, pairSocket protocol.Socket) error {
	message.WriteString(url)
	err := pairSocket.Send(message.Bytes())
	if err != nil {
		return errors.Wrap(err, "failed to send message")
	}
	return nil
}

func handleNodesListRequest(
	ctx context.Context,
	pairSocket protocol.Socket,
	message *bytes.Buffer,
	logger *zap.Logger,
	responsePair chan<- pair.Response,
) error {
	ctx, cancel := context.WithTimeout(ctx, defaultResponseTimeout)
	defer cancel()

	err := pairSocket.Send(message.Bytes())
	if err != nil {
		return errors.Wrap(err, "failed to send message")
	}

	response, err := pairSocket.Recv()
	if err != nil {
		return errors.Wrap(err, "failed to receive message")
	}
	nodeList := pair.NodesListResponse{}
	err = json.Unmarshal(response, &nodeList)
	if err != nil {
		return errors.Wrap(err, "failed to unmarshal message")
	}
	select {
	case responsePair <- &nodeList:
		return nil
	case <-ctx.Done():
		logger.Error("failed to send nodes list response, timeout exceeded",
			zap.Duration("timeout", defaultResponseTimeout),
			zap.ByteString("nodes-status-response", response),
			zap.Error(ctx.Err()),
		)
		return ctx.Err()
	}
}
