package common

type EventType byte

const (
	TimeoutEvnt EventType = iota
	VersionEvnt
	HeightEvnt
	InvalidHeightEvnt
	StateHashEvnt
)

func AddTypeByte(u []byte, event EventType) []byte {
	u = append(u, byte(event))
	return u
}

func ReadTypeByte(msg []byte) ([]byte, EventType) {
	eventByte := msg[len(msg)-1]
	msg = msg[:len(msg)-1]
	return msg, EventType(eventByte)
}
