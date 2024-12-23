package messaging

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/nats-io/nats.go"
	"github.com/pkg/errors"

	"nodemon/pkg/entities"
	"nodemon/pkg/messaging/pair"
)

const defaultResponseTimeout = 5 * time.Second

func StartPairMessagingClient(
	ctx context.Context,
	natsServerURL string,
	requestPair <-chan pair.Request,
	responsePair chan<- pair.Response,
	logger *zap.Logger,
	botRequestsTopic string,
) error {
	nc, err := nats.Connect(natsServerURL, nats.Timeout(nats.DefaultTimeout))
	if err != nil {
		zap.S().Fatalf("Failed to connect to nats server: %v", err)
		return err
	}
	defer nc.Close()

	done := runPairLoop(ctx, requestPair, nc, logger, responsePair, botRequestsTopic)

	<-ctx.Done()
	logger.Info("stopping pair messaging service...")
	<-done
	logger.Info("pair messaging service finished")
	return nil
}

func runPairLoop(
	ctx context.Context,
	requestPair <-chan pair.Request,
	nc *nats.Conn,
	logger *zap.Logger,
	responsePair chan<- pair.Response,
	botRequestsTopic string,
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

				err := handlePairRequest(ctx, request, nc, message, logger, responsePair, botRequestsTopic)
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
	nc *nats.Conn,
	message *bytes.Buffer,
	logger *zap.Logger,
	responsePair chan<- pair.Response,
	botRequestsTopic string,
) error {
	switch r := request.(type) {
	case *pair.NodesListRequest:
		return handleNodesListRequest(ctx, nc, message, logger, responsePair, botRequestsTopic)
	case *pair.InsertNewNodeRequest:
		return handleInsertNewNodeRequest(r.URL, message, nc, botRequestsTopic)
	case *pair.UpdateNodeRequest:
		return handleUpdateNodeRequest(r.URL, r.Alias, message, nc, botRequestsTopic)
	case *pair.DeleteNodeRequest:
		return handleDeleteNodeRequest(r.URL, message, nc, botRequestsTopic)
	case *pair.NodesStatusRequest:
		return handleNodesStatementsRequest(ctx, r.URLs, message, nc, logger, responsePair, botRequestsTopic)
	case *pair.NodeStatementRequest:
		return handleNodesStatementRequest(ctx, r.URL, r.Height, logger, message, nc, responsePair, botRequestsTopic)
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
	nc *nats.Conn,
	responsePair chan<- pair.Response,
	botRequestsTopic string,
) error {
	ctx, cancel := context.WithTimeout(ctx, defaultResponseTimeout)
	defer cancel()

	req, err := json.Marshal(entities.NodeHeight{URL: url, Height: height})
	if err != nil {
		return errors.Wrap(err, "failed to marshal message to pair socket")
	}

	message.Write(req)

	response, err := nc.Request(botRequestsTopic, message.Bytes(), defaultResponseTimeout)
	if err != nil {
		return errors.Wrap(err, "failed to receive message from nodemon")
	}

	nodeStatementResp := pair.NodeStatementResponse{}
	err = json.Unmarshal(response.Data, &nodeStatementResp)
	if err != nil {
		return errors.Wrap(err, "failed to unmarshal message from pair socket")
	}
	select {
	case responsePair <- &nodeStatementResp:
		return nil
	case <-ctx.Done():
		logger.Error("failed to send node statement response, timeout exceeded",
			zap.Duration("timeout", defaultResponseTimeout),
			zap.ByteString("node-statement-response", response.Data),
			zap.Error(ctx.Err()),
		)
		return ctx.Err()
	}
}

func handleNodesStatementsRequest(
	ctx context.Context,
	urls []string,
	message *bytes.Buffer,
	nc *nats.Conn,
	logger *zap.Logger,
	responsePair chan<- pair.Response,
	botRequestsTopic string,
) error {
	ctx, cancel := context.WithTimeout(ctx, defaultResponseTimeout)
	defer cancel()

	message.WriteString(strings.Join(urls, ","))

	response, err := nc.Request(botRequestsTopic, message.Bytes(), defaultResponseTimeout)
	if err != nil {
		return errors.Wrap(err, "failed to receive message from nodemon")
	}
	nodesStatusResp := pair.NodesStatementsResponse{}
	err = json.Unmarshal(response.Data, &nodesStatusResp)
	if err != nil {
		return errors.Wrap(err, "failed to unmarshal message from pair socket")
	}
	select {
	case responsePair <- &nodesStatusResp:
		return nil
	case <-ctx.Done():
		logger.Error("failed to send nodes status response, timeout exceeded",
			zap.Duration("timeout", defaultResponseTimeout),
			zap.ByteString("nodes-status-response", response.Data),
			zap.Error(ctx.Err()),
		)
		return ctx.Err()
	}
}

func handleUpdateNodeRequest(url, alias string, message *bytes.Buffer, nc *nats.Conn, botRequestsTopic string) error {
	node := entities.Node{URL: url, Enabled: true, Alias: alias}
	nodeInfo, err := json.Marshal(node)
	if err != nil {
		return errors.Wrap(err, "failed to marshal node's info")
	}
	message.Write(nodeInfo)
	// ignore a response
	_, err = nc.Request(botRequestsTopic, message.Bytes(), defaultResponseTimeout)
	if err != nil {
		return errors.Wrap(err, "failed to send message")
	}
	return nil
}

func handleDeleteNodeRequest(url string, message *bytes.Buffer, nc *nats.Conn, botRequestsTopic string) error {
	message.WriteString(url)
	// ignore response
	_, err := nc.Request(botRequestsTopic, message.Bytes(), defaultResponseTimeout)
	if err != nil {
		return errors.Wrap(err, "failed to receive message from nodemon")
	}
	return nil
}

func handleInsertNewNodeRequest(url string, message *bytes.Buffer, nc *nats.Conn, botRequestsTopic string) error {
	message.WriteString(url)
	// ignore a response
	_, err := nc.Request(botRequestsTopic, message.Bytes(), defaultResponseTimeout)
	if err != nil {
		return errors.Wrap(err, "failed to send message")
	}
	return nil
}

func handleNodesListRequest(
	ctx context.Context,
	nc *nats.Conn,
	message *bytes.Buffer,
	logger *zap.Logger,
	responsePair chan<- pair.Response,
	botRequestsTopic string,
) error {
	ctx, cancel := context.WithTimeout(ctx, defaultResponseTimeout)
	defer cancel()

	response, err := nc.Request(botRequestsTopic, message.Bytes(), defaultResponseTimeout)
	if err != nil {
		return errors.Wrap(err, "failed to receive message")
	}
	nodeList := pair.NodesListResponse{}
	err = json.Unmarshal(response.Data, &nodeList)
	if err != nil {
		return errors.Wrap(err, "failed to unmarshal message")
	}
	select {
	case responsePair <- &nodeList:
		return nil
	case <-ctx.Done():
		logger.Error("failed to send nodes list response, timeout exceeded",
			zap.Duration("timeout", defaultResponseTimeout),
			zap.ByteString("nodes-status-response", response.Data),
			zap.Error(ctx.Err()),
		)
		return ctx.Err()
	}
}
