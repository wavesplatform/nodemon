package entities

import (
	"fmt"
	"time"

	"github.com/wavesplatform/gowaves/pkg/proto"
)

const (
	SimpleAlertNotificationType        = "SimpleAlert"
	UnreachableAlertNotificationType   = "UnreachableAlert"
	IncompleteAlertNotificationType    = "IncompleteAlert"
	InvalidHeightAlertNotificationType = "InvalidHeightAlert"
	HeightAlertNotificationType        = "HeightAlert"
	StateHashAlertNotificationType     = "StateHashAlert"
)

type Alert interface {
	Notification
	Message() string
	Time() time.Time
	fmt.Stringer
}

type SimpleAlert struct {
	Timestamp   int64
	Description string
}

func (a *SimpleAlert) Type() string {
	return SimpleAlertNotificationType
}

func (a *SimpleAlert) Message() string {
	return a.Description
}

func (a *SimpleAlert) Time() time.Time {
	return time.Unix(a.Timestamp, 0)
}

func (a *SimpleAlert) String() string {
	return fmt.Sprintf("%s: %s", a.Type(), a.Message())
}

type UnreachableAlert struct {
	Timestamp int64  `json:"timestamp"`
	Node      string `json:"node"`
}

func (a *UnreachableAlert) Type() string {
	return UnreachableAlertNotificationType
}

func (a *UnreachableAlert) Message() string {
	return fmt.Sprintf("Node %q is UNREACHABLE at %s", a.Node, a.Time().String())
}

func (a *UnreachableAlert) Time() time.Time {
	return time.Unix(a.Timestamp, 0)
}

func (a *UnreachableAlert) String() string {
	return fmt.Sprintf("%s: %s", a.Type(), a.Message())
}

type IncompleteAlert struct {
	NodeStatement
}

func (a *IncompleteAlert) Type() string {
	return IncompleteAlertNotificationType
}

func (a *IncompleteAlert) Message() string {
	return fmt.Sprintf("Node %q (%s) has incomplete statement info at height %d", a.Node, a.Version, a.Height)
}

func (a *IncompleteAlert) Time() time.Time {
	return time.Unix(a.Timestamp, 0)
}

func (a *IncompleteAlert) String() string {
	return fmt.Sprintf("%s: %s", a.Type(), a.Message())
}

type InvalidHeightAlert struct {
	NodeStatement
}

func (a *InvalidHeightAlert) Type() string {
	return InvalidHeightAlertNotificationType
}

func (a *InvalidHeightAlert) Message() string {
	return fmt.Sprintf("Node %q (%s) has invalid height %d", a.Node, a.Version, a.Height)
}

func (a *InvalidHeightAlert) Time() time.Time {
	return time.Unix(a.Timestamp, 0)
}

func (a *InvalidHeightAlert) String() string {
	return fmt.Sprintf("%s: %s", a.Type(), a.Message())
}

type HeightGroup struct {
	Height int   `json:"height,omitempty"`
	Nodes  Nodes `json:"group,omitempty"`
}

type HeightAlert struct {
	Timestamp        int64       `json:"timestamp"`
	MaxHeightGroup   HeightGroup `json:"max_height_group"`
	OtherHeightGroup HeightGroup `json:"other_height_group"`
}

func (a *HeightAlert) Type() string {
	return HeightAlertNotificationType
}

func (a *HeightAlert) Message() string {
	return fmt.Sprintf("Too big height (%d - %d = %d) diff between nodes groups: max=%v, other=%v",
		a.MaxHeightGroup.Height,
		a.OtherHeightGroup.Height,
		a.MaxHeightGroup.Height-a.OtherHeightGroup.Height,
		a.MaxHeightGroup.Nodes,
		a.OtherHeightGroup.Nodes,
	)
}

func (a *HeightAlert) Time() time.Time {
	return time.Unix(a.Timestamp, 0)
}

func (a *HeightAlert) String() string {
	return fmt.Sprintf("%s: %s", a.Type(), a.Message())
}

type StateHashGroup struct {
	Nodes     Nodes           `json:"nodes,omitempty"`
	StateHash proto.StateHash `json:"state_hash"`
}

type StateHashAlert struct {
	Timestamp                 int64           `json:"timestamp"`
	CurrentGroupsHeight       int             `json:"current_groups_height,omitempty"`
	LastCommonStateHashHeight int             `json:"last_common_state_hash_height,omitempty"`
	LastCommonStateHash       proto.StateHash `json:"last_common_state_hash"`
	FirstGroup                StateHashGroup  `json:"first_group"`
	SecondGroup               StateHashGroup  `json:"second_group"`
}

func (a *StateHashAlert) Type() string {
	return StateHashAlertNotificationType
}

func (a *StateHashAlert) Message() string {
	return fmt.Sprintf(
		"Different state hash between nodes on same height %d: %q=%v, %q=%v. Fork occured after: height %d, statehash %q, blockID %q",
		a.CurrentGroupsHeight,
		a.FirstGroup.StateHash.SumHash.String(),
		a.FirstGroup.Nodes,
		a.SecondGroup.StateHash.SumHash.String(),
		a.SecondGroup.Nodes,
		a.LastCommonStateHashHeight,
		a.LastCommonStateHash.SumHash.String(),
		a.LastCommonStateHash.BlockID.String(),
	)
}

func (a *StateHashAlert) Time() time.Time {
	return time.Unix(a.Timestamp, 0)
}

func (a *StateHashAlert) String() string {
	return fmt.Sprintf("%s: %s", a.Type(), a.Message())
}
