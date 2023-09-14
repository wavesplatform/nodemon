package messaging

import (
	"nodemon/pkg/messaging"

	"go.nanomsg.org/mangos/v3/protocol"
)

type Bot interface {
	SendAlertMessage(msg messaging.AlertMessage)
	SendMessage(msg string)
	SubscribeToAllAlerts() error
	SetSubSocket(subSocket protocol.Socket)
	IsEligibleForAction(chatID string) bool
}
