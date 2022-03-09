package messaging

import (
	"context"
	"log"

	"go.nanomsg.org/mangos/v3"
	"go.nanomsg.org/mangos/v3/protocol/sub"
	_ "go.nanomsg.org/mangos/v3/transport/all"
)

func StartMessagingClient(ctx context.Context, nanomsgURL string, bots *Bots) error {
	socket, err := sub.NewSocket()
	if err != nil {
		log.Printf("failed to get new sub socket: %v", err)
		return err
	}
	if err := socket.Dial(nanomsgURL); err != nil {
		log.Printf("failed to dial on sub socket: %v", err)
		return err
	}
	err = socket.SetOption(mangos.OptionSubscribe, []byte(""))
	if err != nil {
		log.Printf("failed to subscribe on empty topic: %v", err)
		return err
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				msg, err := socket.Recv()
				if err != nil {
					log.Printf("failed to receive message: %v", err)
					return
				}
				bots.TgBotEnvironment.SendMessageTg(msg)
			}
		}
	}()

	<-ctx.Done()
	log.Println("messaging service finished")
	return nil
}
