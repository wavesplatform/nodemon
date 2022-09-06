package pubsub

import (
	"context"
	"log"

	"go.nanomsg.org/mangos/v3/protocol"
	"go.nanomsg.org/mangos/v3/protocol/sub"
	_ "go.nanomsg.org/mangos/v3/transport/all"
	"nodemon/pkg/messaging"
)

func StartSubMessagingClient(ctx context.Context, nanomsgURL string, bots []messaging.Bot) error {
	subSocket, err := sub.NewSocket()
	if err != nil {
		log.Printf("failed to get new sub socket: %v", err)
		return err
	}
	defer func(pubSubSocket protocol.Socket) {
		if err := pubSubSocket.Close(); err != nil {
			log.Printf("Failed to close pair socket: %v", err)
		}
	}(subSocket)

	for _, bot := range bots {
		bot.SetPubSubSocket(subSocket)
	}
	if err := subSocket.Dial(nanomsgURL); err != nil {
		log.Printf("failed to dial on sub socket: %v", err)
		return err
	}
	for _, bot := range bots {
		err = bot.SubscribeToAllAlerts()
		if err != nil {
			log.Printf("failed to subscribe on empty topic: %v", err)
			return err
		}
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				msg, err := subSocket.Recv()
				if err != nil {
					log.Printf("failed to receive message: %v", err)
					return
				}
				for _, bot := range bots {
					bot.SendAlertMessage(msg)
				}
			}
		}
	}()

	<-ctx.Done()
	log.Println("sub messaging service finished")
	return nil
}
