package messaging

import "go.nanomsg.org/mangos/v3/protocol"

type Bot interface {
	SendAlertMessage(msg []byte)
	SendMessage(msg string)
	Start()
	SubscribeToAllAlerts() error
	SetSubSocket(subSocket protocol.Socket)
	IsEligibleForAction(chatID string) bool
}
