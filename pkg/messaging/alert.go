package messaging

import (
	"github.com/pkg/errors"
	"github.com/wavesplatform/gowaves/pkg/crypto"
	"nodemon/pkg/entities"
)

type AlertMessage struct {
	alertType   entities.AlertType
	referenceID crypto.Digest
	data        []byte // it's represented in JSON format
}

func NewAlertMessageFromBytes(msgData []byte) (AlertMessage, error) {
	const minMsgSize = 1 + crypto.DigestSize
	if l := len(msgData); l < minMsgSize {
		return AlertMessage{}, errors.Errorf("message has inssufficient length: want at least %d, got %d", minMsgSize, l)
	}
	referenceID, err := crypto.NewDigestFromBytes(msgData[1 : 1+crypto.DigestSize])
	if err != nil {
		return AlertMessage{}, errors.Wrap(err, "failed to extract alertID from serialized message")
	}
	return AlertMessage{
		alertType:   entities.AlertType(msgData[0]),
		referenceID: referenceID,
		data:        msgData[minMsgSize:],
	}, nil
}

func (a AlertMessage) AlertType() entities.AlertType {
	return a.alertType
}

func (a AlertMessage) ReferenceID() crypto.Digest {
	return a.referenceID
}

func (a AlertMessage) Data() []byte {
	return a.data
}
