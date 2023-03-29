package messaging

import (
	"go.nanomsg.org/mangos/v3/protocol"
	generalMessaging "nodemon/pkg/messaging"
)

type Bot interface {
	SendAlertMessage(msg generalMessaging.AlertMessage)
	SendMessage(msg string)
	Start() error
	SubscribeToAllAlerts() error
	SetSubSocket(subSocket protocol.Socket)
	IsEligibleForAction(chatID string) bool
}
