package messaging

import (
	"github.com/pkg/errors"

	"github.com/wavesplatform/gowaves/pkg/crypto"
	"go.nanomsg.org/mangos/v3/protocol"
	"nodemon/pkg/entities"
)

type AlertMessage struct {
	alertType entities.AlertType
	id        crypto.Digest
	data      []byte // it's represented in JSON format
}

func NewAlertMessageFromBytes(msgData []byte) (AlertMessage, error) {
	const minMsgSize = 1 + crypto.DigestSize
	if l := len(msgData); l < minMsgSize {
		return AlertMessage{}, errors.Errorf("message has inssufficient length: want at least %d, got %d", minMsgSize, l)
	}
	id, err := crypto.NewDigestFromBytes(msgData[1 : crypto.DigestSize+1])
	if err != nil {
		return AlertMessage{}, errors.Wrap(err, "failed to extract alertID from serialized message")
	}
	return AlertMessage{
		alertType: entities.AlertType(msgData[0]),
		id:        id,
		data:      msgData[minMsgSize:],
	}, nil
}

func (a AlertMessage) AlertType() entities.AlertType {
	return a.alertType
}

func (a AlertMessage) ID() crypto.Digest {
	return a.id
}

func (a AlertMessage) Data() []byte {
	return a.data
}

type Bot interface {
	SendAlertMessage(msg AlertMessage)
	SendMessage(msg string)
	Start() error
	SubscribeToAllAlerts() error
	SetSubSocket(subSocket protocol.Socket)
	IsEligibleForAction(chatID string) bool
}
