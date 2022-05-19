package pair

import (
	"bytes"
	"context"
	"encoding/json"

	"go.nanomsg.org/mangos/v3/protocol/pair"
	"log"
)

func StartMessagingPairClient(ctx context.Context, nanomsgURL string, requestPair chan RequestPair, responsePair chan ResponsePair) error {
	socket, err := pair.NewSocket()
	if err != nil {
		log.Printf("failed to get new pair socket: %v", err)
		return err
	}
	if err := socket.Dial(nanomsgURL); err != nil {
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

					err = socket.Send(message.Bytes())
					if err != nil {
						log.Printf("faied to send a request to pair socket, %v", err)
					}

					response, err := socket.Recv()
					if err != nil {
						log.Printf("failed to receive a response from pair socket, %v", err)
					}
					nodeList := NodeListResponse{}
					err = json.Unmarshal(response, &nodeList)
					if err != nil {
						log.Printf("failed to unmarshal response from pair socket, %v", err)
					}
					responsePair <- &nodeList

				case *InsertNewNodeRequest:
					message.WriteByte(byte(RequestInsertNewNodeT))

					message.Write([]byte(r.Url))
					err = socket.Send(message.Bytes())
					if err != nil {
						log.Printf("faied to send a request to pair socket, %v", err)
					}

				case *DeleteNodeRequest:
					message.WriteByte(byte(RequestDeleteNodeT))

					message.Write([]byte(r.Url))
					err = socket.Send(message.Bytes())
					if err != nil {
						log.Printf("faied to send a request to pair socket, %v", err)
					}

				default:
					log.Printf("request type to the pair socket is unknown")

				}

			}
		}
	}()

	<-ctx.Done()
	log.Println("messaging service finished")
	return nil
}
