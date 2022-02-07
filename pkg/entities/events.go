package entities

import "github.com/wavesplatform/gowaves/pkg/proto"

type Event interface {
	Node() string
}

type UnreachableEvent struct {
	node string
}

func NewUnreachableEvent(node string) *UnreachableEvent {
	return &UnreachableEvent{node: node}
}

func (e *UnreachableEvent) Node() string {
	return e.node
}

type VersionEvent struct {
	node string
	v    string
}

func NewVersionEvent(node, ver string) *VersionEvent {
	return &VersionEvent{node: node, v: ver}
}

func (e *VersionEvent) Node() string {
	return e.node
}

func (e *VersionEvent) Version() string {
	return e.v
}

type HeightEvent struct {
	node string
	h    int
}

func NewHeightEvent(node string, height int) *HeightEvent {
	return &HeightEvent{node: node, h: height}
}

func (e *HeightEvent) Node() string {
	return e.node
}

func (e *HeightEvent) Height() int {
	return e.h
}

type InvalidHeightEvent struct {
	node string
	h    int
}

func NewInvalidHeightEvent(node string, height int) *InvalidHeightEvent {
	return &InvalidHeightEvent{node: node, h: height}
}

func (e *InvalidHeightEvent) Node() string {
	return e.node
}

func (e *InvalidHeightEvent) Height() int {
	return e.h
}

type StateHashEvent struct {
	node string
	h    int
	sh   *proto.StateHash
}

func NewStateHashEvent(node string, height int, sh *proto.StateHash) *StateHashEvent {
	return &StateHashEvent{node: node, h: height, sh: sh}
}

func (e *StateHashEvent) Node() string {
	return e.node
}

func (e *StateHashEvent) Height() int {
	return e.h
}

func (e *StateHashEvent) StateHash() *proto.StateHash {
	return e.sh
}
