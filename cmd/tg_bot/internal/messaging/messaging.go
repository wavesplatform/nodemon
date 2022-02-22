package messaging

import (
	"context"
	"go.nanomsg.org/mangos/v3"
	"go.nanomsg.org/mangos/v3/protocol/sub"
	_ "go.nanomsg.org/mangos/v3/transport/all"
	"gopkg.in/telebot.v3"
	"log"
)

type MessageEnvironment struct {
	Chat         *telebot.Chat
	ReceivedChat bool
}

func StartMessagingClient(ctx context.Context, nanomsgURL string, bot *telebot.Bot, botEnv *MessageEnvironment) error {
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
				log.Printf("message received: %s", string(msg))

				if !botEnv.ReceivedChat {
					log.Println("haven't received a chat id yet")
					continue
				}

				_, err = bot.Send(botEnv.Chat, string(msg))
				if err != nil {
					log.Printf("failed to send a message to telegram, %v", err)
				}
			}
		}
	}()

	<-ctx.Done()
	log.Println("messaging service finished")
	return nil
}
