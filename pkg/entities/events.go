package entities

import (
	"github.com/wavesplatform/gowaves/pkg/proto"
)

type EventWithTimestampProducer interface {
	WithTimestamp(ts int64) Event
}

type Event interface {
	Node() string
	Timestamp() int64
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

func (e *UnreachableEvent) Statement() NodeStatement {
	return NodeStatement{
		Node:      e.Node(),
		Timestamp: e.Timestamp(),
		Status:    Unreachable,
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

func (e *VersionEvent) Statement() NodeStatement {
	return NodeStatement{
		Node:      e.Node(),
		Timestamp: e.Timestamp(),
		Status:    Incomplete,
		Version:   e.Version(),
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
}

func NewStateHashEvent(node string, ts int64, v string, h int, sh *proto.StateHash, bt int) *StateHashEvent {
	return &StateHashEvent{node: node, ts: ts, v: v, h: h, sh: sh, baseTarget: bt}
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

func (e *StateHashEvent) Statement() NodeStatement {
	return NodeStatement{
		Node:       e.Node(),
		Timestamp:  e.Timestamp(),
		Status:     OK,
		Version:    e.Version(),
		Height:     e.Height(),
		StateHash:  e.StateHash(),
		BaseTarget: e.BaseTarget(),
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
