package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/pkg/errors"
	"go.nanomsg.org/mangos/v3"
	"go.nanomsg.org/mangos/v3/protocol/sub"
	_ "go.nanomsg.org/mangos/v3/transport/all"
	"gopkg.in/telebot.v3"
	"log"
	"nodemon/pkg/common"
)

type MessageEnvironment struct {
	Chat         *telebot.Chat
	ReceivedChat bool
}

func ReadMessage(msg []byte) (string, error) {
	msg, eventType := common.ReadTypeByte(msg)
	switch eventType {
	case common.TimeoutEvnt:
		event := common.TimeoutEvent{}
		err := json.Unmarshal(msg, &event)
		if err != nil {
			return "", errors.Wrap(err, "failed to unmarshal timeout event")
		}
		return fmt.Sprintf("From node %s: timeout!", event.Node), nil
	case common.VersionEvnt:
		event := &common.VersionEvent{}
		err := json.Unmarshal(msg, &event)
		if err != nil {
			return "", errors.Wrap(err, "failed to unmarshal version event")
		}
		return fmt.Sprintf("From the node %s: the version is %s", event.Node, event.Version), nil
	case common.HeightEvnt:
		event := &common.HeightEvent{}
		err := json.Unmarshal(msg, &event)
		if err != nil {
			return "", errors.Wrap(err, "failed to unmarshal version event")
		}
		return fmt.Sprintf("From the node %s: the height is %d", event.Node, event.Height), nil
	case common.InvalidHeightEvnt:
		event := &common.InvalidHeightEvent{}
		err := json.Unmarshal(msg, &event)
		if err != nil {
			return "", errors.Wrap(err, "failed to unmarshal version event")
		}
		return fmt.Sprintf("From the node %s: invalid height %d!", event.Node, event.Height), nil
	case common.StateHashEvnt:
		event := &common.StateHashEvent{}
		err := json.Unmarshal(msg, &event)
		if err != nil {
			return "", errors.Wrap(err, "failed to unmarshal version event")
		}
		return fmt.Sprintf("From the node %s: on the height %d state hash is %s", event.Node, event.Height, event.StateHash.SumHash.ShortString()), nil // TODO add an opportunity for viewing short and full state hashes
	default:
		return "", errors.New("failed to read byte type: wrong type")
	}

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
				//message, err := ReadMessage(msg) // TODO make the message more user friendly
				//if err != nil {
				//	log.Printf("failed to extract message from nodemon service, %v", err)
				//	return
				//}
				msg, _ = common.ReadTypeByte(msg)

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
