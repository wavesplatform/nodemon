package messaging

import (
	"nodemon/pkg/messaging"
)

type Bot interface {
	SendAlertMessage(msg messaging.AlertMessage)
	SendMessage(msg string)
	SubscribeToAllAlerts() error
	IsEligibleForAction(chatID string) bool
}
