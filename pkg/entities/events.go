package entities

import (
	"github.com/wavesplatform/gowaves/pkg/proto"
)

type Event interface {
	Node() string
	Timestamp() int64
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

type StateHashEvent struct {
	node string
	ts   int64
	v    string
	h    int
	sh   *proto.StateHash
}

func NewStateHashEvent(node string, ts int64, v string, h int, sh *proto.StateHash) *StateHashEvent {
	return &StateHashEvent{node: node, ts: ts, v: v, h: h, sh: sh}
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
