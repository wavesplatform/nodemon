package messaging

import (
	generalMessaging "nodemon/pkg/messaging"

	"go.nanomsg.org/mangos/v3/protocol"
)

type Bot interface {
	SendAlertMessage(msg generalMessaging.AlertMessage)
	SendMessage(msg string)
	Start() error
	SubscribeToAllAlerts() error
	SetSubSocket(subSocket protocol.Socket)
	IsEligibleForAction(chatID string) bool
}
