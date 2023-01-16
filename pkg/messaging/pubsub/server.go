package pubsub

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"

	"github.com/pkg/errors"
	"go.nanomsg.org/mangos/v3/protocol"
	"go.nanomsg.org/mangos/v3/protocol/pub"
	_ "go.nanomsg.org/mangos/v3/transport/all"
	"go.uber.org/zap"
	"nodemon/pkg/entities"
	"nodemon/pkg/messaging"
	"nodemon/pkg/storing/nodes"
)

func StartPubMessagingServer(ctx context.Context, nanomsgURL string, alerts <-chan entities.Alert, logger *zap.Logger, ns *nodes.Storage) error {
	if len(nanomsgURL) == 0 || len(strings.Fields(nanomsgURL)) > 1 {
		return errors.New("invalid nanomsg IPC URL for pub sub socket")
	}

	socketPub, err := pub.NewSocket()
	if err != nil {
		return err
	}
	defer func(socketPub protocol.Socket) {
		if err := socketPub.Close(); err != nil {
			logger.Error("Failed to close pub socket", zap.Error(err))
		}
	}(socketPub)

	if err := socketPub.Listen(nanomsgURL); err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case alert := <-alerts:
			logger.Sugar().Infof("Alert has been generated: %v", alert)

			details, err := replaceNodesWithAliases(ns, alert.Message())
			if err != nil {
				if err != nil {
					logger.Error("Failed to replace nodes with aliases", zap.Error(err))
				}
			}

			jsonAlert, err := json.Marshal(
				messaging.Alert{
					AlertDescription: alert.ShortDescription(),
					Level:            alert.Level(),
					Details:          details,
				})
			if err != nil {
				logger.Error("Failed to marshal alert to json", zap.Error(err))
			}

			message := &bytes.Buffer{}
			message.WriteByte(byte(alert.Type()))
			message.Write(jsonAlert)
			err = socketPub.Send(message.Bytes())
			if err != nil {
				logger.Error("Failed to send alert to socket", zap.Error(err))
			}
		}
	}
}

func replaceNodesWithAliases(ns *nodes.Storage, message string) (string, error) {
	nodes, err := ns.Nodes(false)
	if err != nil {
		return "", err
	}
	specificNodes, err := ns.Nodes(true)
	if err != nil {
		return "", err
	}
	nodes = append(nodes, specificNodes...)

	for _, n := range nodes {
		if n.Alias != "" {
			n.URL = strings.ReplaceAll(n.URL, entities.HttpsScheme+"://", "")
			n.URL = strings.ReplaceAll(n.URL, entities.HttpScheme+"://", "")
			message = strings.ReplaceAll(message, n.URL, n.Alias)
		}
	}
	return message, nil
}
