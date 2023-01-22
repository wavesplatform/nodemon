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

			jsonAlert, err := AlertToJson(alert)
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

func AlertToJson(alert entities.Alert) ([]byte, error) {
	var jsonAlert []byte
	var err error
	switch a := alert.(type) {
	case *entities.UnreachableAlert:
		jsonAlert, err = json.Marshal(a)
		if err != nil {
			return nil, err
		}
	case *entities.IncompleteAlert:
		jsonAlert, err = json.Marshal(a)
		if err != nil {
			return nil, err
		}
	case *entities.InvalidHeightAlert:
		jsonAlert, err = json.Marshal(a)
		if err != nil {
			return nil, err
		}
	case *entities.HeightAlert:
		jsonAlert, err = json.Marshal(a)
		if err != nil {
			return nil, err
		}
	case *entities.StateHashAlert:
		jsonAlert, err = json.Marshal(a)
		if err != nil {
			return nil, err
		}
	case *entities.AlertFixed:
		jsonAlert, err = json.Marshal(a)
		if err != nil {
			return nil, err
		}
	case *entities.BaseTargetAlert:
		jsonAlert, err = json.Marshal(a)
		if err != nil {
			return nil, err
		}
	case *entities.InternalErrorAlert:
		jsonAlert, err = json.Marshal(a)
		if err != nil {
			return nil, err
		}
	default:
		return nil, errors.New("unknown alert type")
	}
	return jsonAlert, nil
}
