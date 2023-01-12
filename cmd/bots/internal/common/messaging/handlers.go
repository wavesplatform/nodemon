package messaging

import (
	"fmt"
	"sort"

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

func UpdateAliasHandler(
	chatID string,
	bot Bot,
	requestType chan<- pair.RequestPair,
	url string,
	alias string) (string, error) {

	if !bot.IsEligibleForAction(chatID) {
		return insufficientPermissionMsg, InsufficientPermissionsError
	}

	updatedUrl, err := entities.CheckAndUpdateURL(url)
	if err != nil {
		return incorrectUrlMsg, IncorrectUrlError
	}
	requestType <- &pair.UpdateNodeRequest{Url: updatedUrl, Alias: alias}

	return fmt.Sprintf("Node '%s' was updated with alias %s", updatedUrl, alias), nil
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

func RequestNodesStatus(
	requestType chan<- pair.RequestPair,
	responsePairType <-chan pair.ResponsePair,
	urls []string) (*pair.NodesStatusResponse, error) {

	requestType <- &pair.NodesStatusRequest{Urls: urls}
	responsePair := <-responsePairType
	nodesStatus, ok := responsePair.(*pair.NodesStatusResponse)
	if !ok {
		return nil, errors.New("failed to convert response interface to the nodes status type")
	}

	return nodesStatus, nil

}

func RequestNodesList(requestType chan<- pair.RequestPair, responsePairType <-chan pair.ResponsePair, specific bool) ([]string, error) {
	requestType <- &pair.NodesListRequest{Specific: specific}
	responsePair := <-responsePairType
	nodesList, ok := responsePair.(*pair.NodesListResponse)
	if !ok {
		return nil, errors.New("failed to convert response interface to the node list type")
	}
	urls := make([]string, len(nodesList.Nodes))
	for i, n := range nodesList.Nodes {
		urls[i] = n.URL
	}
	sort.Strings(urls)
	return urls, nil
}

func RequestFullNodesList(requestType chan<- pair.RequestPair, responsePairType <-chan pair.ResponsePair, specific bool) ([]entities.Node, error) {
	requestType <- &pair.NodesListRequest{Specific: specific}
	responsePair := <-responsePairType
	nodesList, ok := responsePair.(*pair.NodesListResponse)
	if !ok {
		return nil, errors.New("failed to convert response interface to the node list type")
	}
	return nodesList.Nodes, nil
}

func RequestNodeStatement(requestType chan<- pair.RequestPair, responsePairType <-chan pair.ResponsePair, node string, height int) (*pair.NodeStatementResponse, error) {
	requestType <- &pair.NodeStatementRequest{Url: node, Height: height}
	responsePair := <-responsePairType
	nodeStatementResp, ok := responsePair.(*pair.NodeStatementResponse)
	if !ok {
		return nil, errors.New("failed to convert response interface to the node list type")
	}

	return nodeStatementResp, nil
}
