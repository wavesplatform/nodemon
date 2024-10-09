package entities

import (
	"github.com/wavesplatform/gowaves/pkg/proto"
)

const (
	UndefinedHeight = 0
)

type EventProducerWithTimestamp interface {
	Node() string
	WithTimestamp(ts int64) Event
}

type Event interface {
	Node() string
	Timestamp() int64
	Height() uint64
	Statement() NodeStatement
}

type UnreachableEvent struct {
	node string
	ts   int64
}

func NewUnreachableEvent(node string, ts int64) *UnreachableEvent {
	return &UnreachableEvent{node: node, ts: ts}
}

func (e *UnreachableEvent) Node() string {
	return e.node
}

func (e *UnreachableEvent) Timestamp() int64 {
	return e.ts
}

func (e *UnreachableEvent) Height() uint64 {
	return UndefinedHeight
}

func (e *UnreachableEvent) Statement() NodeStatement {
	return NodeStatement{
		Node:      e.Node(),
		Timestamp: e.Timestamp(),
		Status:    Unreachable,
		Height:    e.Height(),
	}
}

func (e *UnreachableEvent) WithTimestamp(ts int64) Event {
	cpy := *e
	cpy.ts = ts
	return &cpy
}

type VersionEvent struct {
	node string
	ts   int64
	v    string
}

func NewVersionEvent(node string, ts int64, ver string) *VersionEvent {
	return &VersionEvent{node: node, ts: ts, v: ver}
}

func (e *VersionEvent) Node() string {
	return e.node
}

func (e *VersionEvent) Timestamp() int64 {
	return e.ts
}

func (e *VersionEvent) Version() string {
	return e.v
}

func (e *VersionEvent) Height() uint64 {
	return UndefinedHeight
}

func (e *VersionEvent) Statement() NodeStatement {
	return NodeStatement{
		Node:      e.Node(),
		Timestamp: e.Timestamp(),
		Status:    Incomplete,
		Version:   e.Version(),
		Height:    e.Height(),
	}
}

func (e *VersionEvent) WithTimestamp(ts int64) Event {
	cpy := *e
	cpy.ts = ts
	return &cpy
}

type HeightEvent struct {
	VersionEvent
	h uint64
}

func NewHeightEvent(node string, ts int64, v string, height uint64) *HeightEvent {
	return &HeightEvent{VersionEvent: *NewVersionEvent(node, ts, v), h: height}
}

func (e *HeightEvent) Height() uint64 {
	return e.h
}

func (e *HeightEvent) Statement() NodeStatement {
	s := e.VersionEvent.Statement()
	s.Status = Incomplete
	s.Height = e.Height()
	return s
}

func (e *HeightEvent) WithTimestamp(ts int64) Event {
	cpy := *e
	cpy.ts = ts
	return &cpy
}

type InvalidHeightEvent struct {
	HeightEvent
}

func NewInvalidHeightEvent(node string, ts int64, v string, height uint64) *InvalidHeightEvent {
	return &InvalidHeightEvent{HeightEvent: *NewHeightEvent(node, ts, v, height)}
}

func (e *InvalidHeightEvent) Statement() NodeStatement {
	s := e.HeightEvent.Statement()
	s.Status = InvalidHeight
	return s
}

func (e *InvalidHeightEvent) WithTimestamp(ts int64) Event {
	cpy := *e
	cpy.ts = ts
	return &cpy
}

type BlockHeaderEvent struct {
	HeightEvent
	blockID    *proto.BlockID
	generator  *proto.WavesAddress
	challenged bool
}

func NewBlockHeaderEvent(
	node string,
	ts int64,
	v string,
	h uint64,
	blockID *proto.BlockID,
	generator *proto.WavesAddress,
	challenged bool,
) *BlockHeaderEvent {
	return &BlockHeaderEvent{
		HeightEvent: *NewHeightEvent(node, ts, v, h),
		blockID:     blockID,
		generator:   generator,
		challenged:  challenged,
	}
}

func (e *BlockHeaderEvent) BlockID() *proto.BlockID {
	return e.blockID
}

func (e *BlockHeaderEvent) Generator() *proto.WavesAddress {
	return e.generator
}

func (e *BlockHeaderEvent) Challenged() bool {
	return e.challenged
}

func (e *BlockHeaderEvent) Statement() NodeStatement {
	s := e.HeightEvent.Statement()
	s.Status = Incomplete
	s.BlockID = e.BlockID()
	s.Generator = e.Generator()
	s.Challenged = e.Challenged()
	return s
}

func (e *BlockHeaderEvent) WithTimestamp(ts int64) Event {
	cpy := *e
	cpy.ts = ts
	return &cpy
}

type BaseTargetEvent struct {
	BlockHeaderEvent
	bs uint64
}

func NewBaseTargetEvent(
	node string,
	ts int64,
	v string,
	h, bt uint64,
	blockID *proto.BlockID,
	generator *proto.WavesAddress,
	challenged bool,
) *BaseTargetEvent {
	return &BaseTargetEvent{
		BlockHeaderEvent: *NewBlockHeaderEvent(node, ts, v, h, blockID, generator, challenged),
		bs:               bt,
	}
}

func (e *BaseTargetEvent) BaseTarget() uint64 {
	return e.bs
}

func (e *BaseTargetEvent) Statement() NodeStatement {
	s := e.BlockHeaderEvent.Statement()
	s.Status = Incomplete
	s.BaseTarget = e.BaseTarget()
	return s
}

func (e *BaseTargetEvent) WithTimestamp(ts int64) Event {
	cpy := *e
	cpy.ts = ts
	return &cpy
}

type StateHashEvent struct {
	BaseTargetEvent
	sh *proto.StateHash
}

func NewStateHashEvent(
	node string,
	ts int64,
	v string,
	h uint64,
	sh *proto.StateHash,
	bt uint64,
	blockID *proto.BlockID,
	generator *proto.WavesAddress,
	challenged bool,
) *StateHashEvent {
	return &StateHashEvent{
		BaseTargetEvent: *NewBaseTargetEvent(node, ts, v, h, bt, blockID, generator, challenged),
		sh:              sh,
	}
}

func (e *StateHashEvent) StateHash() *proto.StateHash {
	return e.sh
}

func (e *StateHashEvent) Statement() NodeStatement {
	s := e.BaseTargetEvent.Statement()
	s.Status = OK
	s.StateHash = e.StateHash()
	return s
}

func (e *StateHashEvent) WithTimestamp(ts int64) Event {
	cpy := *e
	cpy.ts = ts
	return &cpy
}
