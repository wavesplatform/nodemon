package common

import (
	"github.com/wavesplatform/gowaves/pkg/proto"
)

type EventType byte

const (
	TimeoutEvnt EventType = iota
	VersionEvnt
	HeightEvnt
	InvalidHeightEvnt
	StateHashEvnt
)

type TimeoutEvent struct {
	Node string
}

type VersionEvent struct {
	Node    string
	Version string
}

type HeightEvent struct {
	Node   string
	Height int
}

type InvalidHeightEvent struct {
	Node   string
	Height int
}

type StateHashEvent struct {
	Node      string
	Height    int
	StateHash *proto.StateHash
}

func AddTypeByte(u []byte, event EventType) []byte {
	u = append(u, byte(event))
	return u
}

func ReadTypeByte(msg []byte) ([]byte, EventType) {
	eventByte := msg[len(msg)-1]
	msg = msg[:len(msg)-1]
	return msg, EventType(eventByte)
}
