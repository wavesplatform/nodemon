package pair

import "fmt"

type RequestPairType byte

const (
	RequestNodeListT RequestPairType = iota
	RequestInsertNewNodeT
	RequestDeleteNodeT
)

type RequestPair interface {
	msgRequest() string
}

type NodeListRequest struct {
}

func (nl *NodeListRequest) msgRequest() string {
	return ""
}

type InsertNewNodeRequest struct {
	Url string
}

func (nl *InsertNewNodeRequest) msgRequest() string {
	return ""
}

type DeleteNodeRequest struct {
	Url string
}

func (nl *DeleteNodeRequest) msgRequest() string {
	return ""
}

type ResponsePair interface {
	MsgResult() string
}

type NodeListResponse struct {
	Urls []string `json:"urls"`
}

func (nl *NodeListResponse) MsgResult() string {
	return ""
}

type InsertNewNodeResponse struct {
	url string
}

func (nl *InsertNewNodeResponse) MsgResult() string {
	return fmt.Sprintf("New node at '%s' stored", nl.url)
}

type DeleteNodeResponse struct {
}

func (nl *DeleteNodeResponse) MsgResult() string {
	return ""
}
