package pair

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"strings"

	"go.nanomsg.org/mangos/v3/protocol"
	"go.nanomsg.org/mangos/v3/protocol/pair"
)

func StartPairMessagingClient(ctx context.Context, nanomsgURL string, requestPair chan RequestPair, responsePair chan ResponsePair) error {
	pairSocket, err := pair.NewSocket()
	if err != nil {
		log.Printf("failed to get new pair socket: %v", err)
		return err
	}

	defer func(pairSocket protocol.Socket) {
		if err := pairSocket.Close(); err != nil {
			log.Printf("Failed to close pair socket: %v", err)
		}
	}(pairSocket)

	if err := pairSocket.Dial(nanomsgURL); err != nil {
		log.Printf("failed to dial on pair socket: %v", err)
		return err
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				request := <-requestPair

				message := &bytes.Buffer{}

				switch r := request.(type) {
				case *NodeListRequest:
					message.WriteByte(byte(RequestNodeListT))

					err = pairSocket.Send(message.Bytes())
					if err != nil {
						log.Printf("faied to send a request to pair socket, %v", err)
					}

					response, err := pairSocket.Recv()
					if err != nil {
						log.Printf("failed to receive a response from pair socket, %v", err)
					}
					nodeList := NodesListResponse{}
					err = json.Unmarshal(response, &nodeList)
					if err != nil {
						log.Printf("failed to unmarshal response from pair socket, %v", err)
					}
					responsePair <- &nodeList

				case *InsertNewNodeRequest:
					message.WriteByte(byte(RequestInsertNewNodeT))

					message.Write([]byte(r.Url))
					err = pairSocket.Send(message.Bytes())
					if err != nil {
						log.Printf("faied to send a request to pair socket, %v", err)
					}

				case *DeleteNodeRequest:
					message.WriteByte(byte(RequestDeleteNodeT))

					message.Write([]byte(r.Url))
					err = pairSocket.Send(message.Bytes())
					if err != nil {
						log.Printf("faied to send a request to pair socket, %v", err)
					}
				case *NodesStatusRequest:
					message.WriteByte(byte(RequestNodesStatus))

					message.Write([]byte(strings.Join(r.Urls, ",")))
					err = pairSocket.Send(message.Bytes())
					if err != nil {
						log.Printf("faied to send a request to pair socket, %v", err)
					}

					response, err := pairSocket.Recv()
					if err != nil {
						log.Printf("failed to receive a response from pair socket, %v", err)
					}
					nodesStatusResp := NodesStatusResponse{}
					err = json.Unmarshal(response, &nodesStatusResp)
					if err != nil {
						log.Printf("failed to unmarshal response from pair socket, %v", err)
					}
					responsePair <- &nodesStatusResp

				default:
					log.Printf("request type to the pair socket is unknown: %T", r)

				}

			}
		}
	}()

	<-ctx.Done()
	log.Println("pair messaging service finished")
	return nil
}
