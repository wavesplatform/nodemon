package messaging

type Bots []Bot

func NewBots(bots ...Bot) Bots {
	return bots
}

type Bot interface {
	SendMessage(msg []byte)
	Start()
}
