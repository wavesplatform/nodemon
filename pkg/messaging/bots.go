package messaging

type Bot interface {
	SendMessage(msg []byte)
	Start()
}
