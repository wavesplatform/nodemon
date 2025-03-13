package messaging

import (
	"context"
	stderrs "errors"
	"fmt"

	"nodemon/pkg/entities"
	"nodemon/pkg/messaging/pair"

	"github.com/pkg/errors"
)

const (
	insufficientPermissionMsg = "Sorry, you have no right to add a new node"
	incorrectURLMsg           = "Sorry, the url seems to be incorrect"
)

var (
	ErrInsufficientPermissions = errors.New("insufficient permissions")
	ErrIncorrectURL            = errors.New("incorrect url")
)

func AddNewNodeHandler(
	chatID string,
	bot Bot,
	requestChan chan<- pair.Request,
	url string,
	specific bool,
) (string, error) {
	if !bot.IsEligibleForAction(chatID) {
		return insufficientPermissionMsg, ErrInsufficientPermissions
	}

	updatedURL, err := entities.CheckAndUpdateURL(url)
	if err != nil {
		return incorrectURLMsg, stderrs.Join(ErrIncorrectURL, err)
	}
	requestChan <- &pair.InsertNewNodeRequest{URL: updatedURL, Specific: specific}

	if specific {
		return fmt.Sprintf("New specific node '%s' was added", updatedURL), nil
	}
	return fmt.Sprintf("New node '%s' was added", updatedURL), nil
}

func UpdateAliasHandler(
	chatID string,
	bot Bot,
	requestChan chan<- pair.Request,
	url, alias string,
) (string, error) {
	if !bot.IsEligibleForAction(chatID) {
		return insufficientPermissionMsg, ErrInsufficientPermissions
	}

	updatedURL, err := entities.CheckAndUpdateURL(url)
	if err != nil {
		return incorrectURLMsg, stderrs.Join(ErrIncorrectURL, err)
	}
	requestChan <- &pair.UpdateNodeRequest{URL: updatedURL, Alias: alias}

	return fmt.Sprintf("Node '%s' was updated with alias %s", updatedURL, alias), nil
}

func RemoveNodeHandler(chatID string, bot Bot, requestType chan<- pair.Request, url string) (string, error) {
	if !bot.IsEligibleForAction(chatID) {
		return insufficientPermissionMsg, ErrInsufficientPermissions
	}

	updatedURL, err := entities.CheckAndUpdateURL(url)
	if err != nil {
		return incorrectURLMsg, stderrs.Join(ErrIncorrectURL, err)
	}
	requestType <- &pair.DeleteNodeRequest{URL: updatedURL}

	return fmt.Sprintf("Node '%s' was deleted", url), nil
}

func RequestNodesStatements(
	requestChan chan<- pair.Request,
	responseChan <-chan pair.Response,
	urls []string,
) (*pair.NodesStatementsResponse, error) {
	requestChan <- &pair.NodesStatusRequest{URLs: urls}
	response := <-responseChan
	nodesStatements, ok := response.(*pair.NodesStatementsResponse)
	if !ok {
		return nil, errors.New("failed to convert response interface to the nodes status type")
	}
	return nodesStatements, nil
}

func RequestNodes(
	requestChan chan<- pair.Request,
	responseChan <-chan pair.Response,
	specific bool,
) ([]entities.Node, error) {
	return RequestNodesWithCtx(context.Background(), requestChan, responseChan, specific)
}

func RequestNodesWithCtx(
	ctx context.Context,
	requestChan chan<- pair.Request,
	responseChan <-chan pair.Response,
	specific bool,
) ([]entities.Node, error) {
	requestChan <- &pair.NodesListRequest{Specific: specific}
	select {
	case response := <-responseChan:
		nodesList, ok := response.(*pair.NodesListResponse)
		if !ok {
			return nil, errors.New("failed to convert response interface to the node list type")
		}
		return nodesList.Nodes, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func RequestAllNodes(requestChan chan<- pair.Request, responseChan <-chan pair.Response) ([]entities.Node, error) {
	regularNodes, err := RequestNodes(requestChan, responseChan, false)
	if err != nil {
		return nil, err
	}
	specificNodes, err := RequestNodes(requestChan, responseChan, true)
	if err != nil {
		return nil, err
	}
	nodes := make([]entities.Node, 0, len(regularNodes)+len(specificNodes))
	nodes = append(nodes, regularNodes...)
	nodes = append(nodes, specificNodes...)
	return nodes, nil
}

func NodesToUrls(nodes []entities.Node) []string {
	urls := make([]string, len(nodes))
	for i, n := range nodes {
		urls[i] = n.URL
	}
	return urls
}

func RequestNodeStatement(
	requestChan chan<- pair.Request,
	responseChan <-chan pair.Response,
	node string,
	height int,
) (*pair.NodeStatementResponse, error) {
	requestChan <- &pair.NodeStatementRequest{URL: node, Height: height}
	response := <-responseChan
	nodeStatementResp, ok := response.(*pair.NodeStatementResponse)
	if !ok {
		return nil, errors.New("failed to convert response interface to the node list type")
	}
	return nodeStatementResp, nil
}
