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
	Height() int
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

func (e *UnreachableEvent) Height() int {
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

func (e *VersionEvent) Height() int {
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
	node string
	ts   int64
	v    string
	h    int
}

func NewHeightEvent(node string, ts int64, v string, height int) *HeightEvent {
	return &HeightEvent{node: node, ts: ts, v: v, h: height}
}

func (e *HeightEvent) Node() string {
	return e.node
}

func (e *HeightEvent) Timestamp() int64 {
	return e.ts
}

func (e *HeightEvent) Version() string {
	return e.v
}

func (e *HeightEvent) Height() int {
	return e.h
}

func (e *HeightEvent) Statement() NodeStatement {
	return NodeStatement{
		Node:      e.Node(),
		Timestamp: e.Timestamp(),
		Status:    Incomplete,
		Version:   e.Version(),
		Height:    e.Height(),
	}
}

func (e *HeightEvent) WithTimestamp(ts int64) Event {
	cpy := *e
	cpy.ts = ts
	return &cpy
}

type InvalidHeightEvent struct {
	node string
	ts   int64
	v    string
	h    int
}

func NewInvalidHeightEvent(node string, ts int64, v string, height int) *InvalidHeightEvent {
	return &InvalidHeightEvent{node: node, ts: ts, v: v, h: height}
}

func (e *InvalidHeightEvent) Node() string {
	return e.node
}

func (e *InvalidHeightEvent) Timestamp() int64 {
	return e.ts
}

func (e *InvalidHeightEvent) Version() string {
	return e.v
}

func (e *InvalidHeightEvent) Height() int {
	return e.h
}

func (e *InvalidHeightEvent) Statement() NodeStatement {
	return NodeStatement{
		Node:      e.Node(),
		Timestamp: e.Timestamp(),
		Status:    InvalidHeight,
		Version:   e.Version(),
		Height:    e.Height(),
	}
}

func (e *InvalidHeightEvent) WithTimestamp(ts int64) Event {
	cpy := *e
	cpy.ts = ts
	return &cpy
}

type StateHashEvent struct {
	node       string
	ts         int64
	v          string
	h          int
	sh         *proto.StateHash
	baseTarget int
	blockID    proto.BlockID
	generator  proto.WavesAddress
}

func NewStateHashEvent(node string, ts int64, v string, h int, sh *proto.StateHash, bt int,
	blockID proto.BlockID, generator proto.WavesAddress) *StateHashEvent {
	return &StateHashEvent{node: node, ts: ts, v: v, h: h, sh: sh, baseTarget: bt, blockID: blockID, generator: generator}
}

func (e *StateHashEvent) Node() string {
	return e.node
}

func (e *StateHashEvent) Timestamp() int64 {
	return e.ts
}

func (e *StateHashEvent) Version() string {
	return e.v
}

func (e *StateHashEvent) Height() int {
	return e.h
}

func (e *StateHashEvent) StateHash() *proto.StateHash {
	return e.sh
}

func (e *StateHashEvent) BaseTarget() int {
	return e.baseTarget
}

func (e *StateHashEvent) BlockID() proto.BlockID {
	return e.blockID
}

func (e *StateHashEvent) Generator() proto.WavesAddress {
	return e.generator
}

func (e *StateHashEvent) Statement() NodeStatement {
	return NodeStatement{
		Node:       e.Node(),
		Timestamp:  e.Timestamp(),
		Status:     OK,
		Version:    e.Version(),
		Height:     e.Height(),
		StateHash:  e.StateHash(),
		BaseTarget: e.BaseTarget(),
		BlockID:    e.blockID,
		Generator:  e.generator,
	}
}

func (e *StateHashEvent) WithTimestamp(ts int64) Event {
	cpy := *e
	cpy.ts = ts
	return &cpy
}

type BaseTargetEvent struct {
	node       string
	ts         int64
	v          string
	h          int
	baseTarget int
}

func NewBaseTargetEvent(node string, ts int64, v string, h int, baseTarget int) *BaseTargetEvent {
	return &BaseTargetEvent{node: node, ts: ts, v: v, h: h, baseTarget: baseTarget}
}

func (e *BaseTargetEvent) Node() string {
	return e.node
}

func (e *BaseTargetEvent) Timestamp() int64 {
	return e.ts
}

func (e *BaseTargetEvent) Version() string {
	return e.v
}

func (e *BaseTargetEvent) Height() int {
	return e.h
}

func (e *BaseTargetEvent) BaseTarget() int {
	return e.baseTarget
}

func (e *BaseTargetEvent) Statement() NodeStatement {
	return NodeStatement{
		Node:       e.Node(),
		Timestamp:  e.Timestamp(),
		Status:     Incomplete,
		Version:    e.Version(),
		Height:     e.Height(),
		BaseTarget: e.BaseTarget(),
	}
}

func (e *BaseTargetEvent) WithTimestamp(ts int64) Event {
	cpy := *e
	cpy.ts = ts
	return &cpy
}

type BlockGeneratorEvent struct {
	node      string
	ts        int64
	v         string
	h         int
	bs        int
	blockID   proto.BlockID
	generator proto.WavesAddress
}

func NewBlockGeneratorEvent(node string, ts int64, v string, h int, bs int,
	blockID proto.BlockID, generator proto.WavesAddress) *BlockGeneratorEvent {
	return &BlockGeneratorEvent{node: node, ts: ts, v: v, h: h, bs: bs, blockID: blockID, generator: generator}
}

func (e *BlockGeneratorEvent) Node() string {
	return e.node
}

func (e *BlockGeneratorEvent) Timestamp() int64 {
	return e.ts
}

func (e *BlockGeneratorEvent) Version() string {
	return e.v
}

func (e *BlockGeneratorEvent) Height() int {
	return e.h
}

func (e *BlockGeneratorEvent) BaseTarget() int {
	return e.bs
}

func (e *BlockGeneratorEvent) BlockID() proto.BlockID {
	return e.blockID
}

func (e *BlockGeneratorEvent) Generator() proto.WavesAddress {
	return e.generator
}

func (e *BlockGeneratorEvent) Statement() NodeStatement {
	return NodeStatement{
		Node:       e.Node(),
		Timestamp:  e.Timestamp(),
		Status:     Incomplete,
		Version:    e.Version(),
		Height:     e.Height(),
		BaseTarget: e.BaseTarget(),
		BlockID:    e.BlockID(),
		Generator:  e.Generator(),
	}
}

func (e *BlockGeneratorEvent) WithTimestamp(ts int64) Event {
	cpy := *e
	cpy.ts = ts
	return &cpy
}
