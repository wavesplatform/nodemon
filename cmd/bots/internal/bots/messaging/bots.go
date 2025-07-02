package messaging

import (
	"nodemon/pkg/messaging"

	"github.com/nats-io/nats.go"
)

type Bot interface {
	SendAlertMessage(msg messaging.AlertMessage)
	SendMessage(msg string)
	SetNatsConnection(nc *nats.Conn)
	SetAlertHandlerFunc(alertHandlerFunc func(msg *nats.Msg))
	SubscribeToAllAlerts() error
	IsEligibleForAction(chatID string) bool
}
