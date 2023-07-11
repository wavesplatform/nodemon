package messaging

import (
	"encoding/json"

	"nodemon/pkg/entities"

	"github.com/pkg/errors"
	"github.com/wavesplatform/gowaves/pkg/crypto"
)

type AlertMessage struct {
	alertType   entities.AlertType
	referenceID crypto.Digest
	data        []byte // it's represented in JSON format
}

func NewAlertMessageFromAlert(alert entities.Alert) (AlertMessage, error) {
	jsonAlert, err := json.Marshal(alert)
	if err != nil {
		return AlertMessage{}, errors.Wrapf(err, "failed to marshal alert '%T' to JSON", alert)
	}
	var referenceID crypto.Digest
	switch a := alert.(type) {
	case *entities.AlertFixed:
		referenceID = a.Fixed.ID()
	default:
		referenceID = a.ID()
	}
	return AlertMessage{
		alertType:   alert.Type(),
		referenceID: referenceID,
		data:        jsonAlert,
	}, nil
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

func (a AlertMessage) MarshalBinary() ([]byte, error) {
	data := make([]byte, 0, 1+crypto.DigestSize+len(a.data))
	data = append(data, byte(a.alertType))
	data = append(data, a.referenceID[:]...)
	data = append(data, a.data...)
	return data, nil
}
