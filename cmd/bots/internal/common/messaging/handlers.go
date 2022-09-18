package messaging

import (
	"fmt"

	"github.com/pkg/errors"
	"nodemon/pkg/entities"
	"nodemon/pkg/messaging/pair"
)

var (
	insufficientPermissionMsg = "Sorry, you have no right to add a new node"
	incorrectUrlMsg           = "Sorry, the url seems to be incorrect"
)

var (
	InsufficientPermissionsError = errors.New("insufficient permissions")
	IncorrectUrlError            = errors.New("incorrect url")
)

func AddNewNodeHandler(
	chatID string,
	bot Bot,
	requestType chan<- pair.RequestPair,
	url string,
	specific bool) (string, error) {

	if !bot.IsEligibleForAction(chatID) {
		return insufficientPermissionMsg, InsufficientPermissionsError
	}

	updatedUrl, err := entities.CheckAndUpdateURL(url)
	if err != nil {
		return incorrectUrlMsg, IncorrectUrlError
	}
	requestType <- &pair.InsertNewNodeRequest{Url: updatedUrl, Specific: specific}

	if specific {
		return fmt.Sprintf("New specific node '%s' was added", updatedUrl), nil
	}

	return fmt.Sprintf("New node '%s' was added", updatedUrl), nil
}

func RemoveNodeHandler(
	chatID string,
	bot Bot,
	requestType chan<- pair.RequestPair,
	url string) (string, error) {

	if !bot.IsEligibleForAction(chatID) {
		return insufficientPermissionMsg, InsufficientPermissionsError
	}

	updatedUrl, err := entities.CheckAndUpdateURL(url)
	if err != nil {
		return incorrectUrlMsg, IncorrectUrlError
	}
	requestType <- &pair.DeleteNodeRequest{Url: updatedUrl}

	return fmt.Sprintf("Node '%s' was deleted", url), nil
}
