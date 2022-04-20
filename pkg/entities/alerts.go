package entities

import (
	"bytes"
	"fmt"
	"strconv"
	"time"

	"github.com/wavesplatform/gowaves/pkg/crypto"
	"github.com/wavesplatform/gowaves/pkg/proto"
)

const (
	SimpleAlertNotificationType        = "SimpleAlert"
	UnreachableAlertNotificationType   = "UnreachableAlert"
	IncompleteAlertNotificationType    = "IncompleteAlert"
	InvalidHeightAlertNotificationType = "InvalidHeightAlert"
	HeightAlertNotificationType        = "HeightAlert"
	StateHashAlertNotificationType     = "StateHashAlert"
	AlertFixedNotificationType         = "AlertFixed"
)

type Alert interface {
	Notification
	ID() string
	Message() string
	Time() time.Time
	fmt.Stringer
}

type SimpleAlert struct {
	Timestamp   int64  `json:"timestamp"`
	Description string `json:"description"`
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

func (a *SimpleAlert) ID() string {
	digest := crypto.MustFastHash([]byte(a.Type() + a.Description))
	return digest.String()
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
	return fmt.Sprintf("Node %q is UNREACHABLE", a.Node)
}

func (a *UnreachableAlert) Time() time.Time {
	return time.Unix(a.Timestamp, 0)
}

func (a *UnreachableAlert) ID() string {
	digest := crypto.MustFastHash([]byte(a.Type() + a.Node))
	return digest.String()
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

func (a *IncompleteAlert) ID() string {
	digest := crypto.MustFastHash([]byte(a.Type() + a.Node))
	return digest.String()
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

func (a *InvalidHeightAlert) ID() string {
	digest := crypto.MustFastHash([]byte(a.Type() + a.Node))
	return digest.String()
}

type HeightGroup struct {
	Height int   `json:"height"`
	Nodes  Nodes `json:"group"`
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

func (a *HeightAlert) ID() string {
	var buff bytes.Buffer
	buff.WriteString(a.Type())

	for _, node := range a.MaxHeightGroup.Nodes {
		buff.WriteString(node)
	}
	for _, node := range a.OtherHeightGroup.Nodes {
		buff.WriteString(node)
	}
	buff.WriteString(strconv.Itoa(a.OtherHeightGroup.Height))

	digest := crypto.MustFastHash(buff.Bytes())
	return digest.String()
}

type StateHashGroup struct {
	Nodes     Nodes           `json:"nodes"`
	StateHash proto.StateHash `json:"state_hash"`
}

type StateHashAlert struct {
	Timestamp                 int64           `json:"timestamp"`
	CurrentGroupsHeight       int             `json:"current_groups_height"`
	LastCommonStateHashExist  bool            `json:"last_common_state_hash_exist"`
	LastCommonStateHashHeight int             `json:"last_common_state_hash_height"` // can be empty if LastCommonStateHashExist == false
	LastCommonStateHash       proto.StateHash `json:"last_common_state_hash"`        /// can be empty if LastCommonStateHashExist == false
	FirstGroup                StateHashGroup  `json:"first_group"`
	SecondGroup               StateHashGroup  `json:"second_group"`
}

func (a *StateHashAlert) Type() string {
	return StateHashAlertNotificationType
}

func (a *StateHashAlert) Message() string {
	if a.LastCommonStateHashExist {
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
	return fmt.Sprintf(
		"Different state hash between nodes on same height %d: %q=%v, %q=%v. Failed to find last common state hash and blockID",
		a.CurrentGroupsHeight,
		a.FirstGroup.StateHash.SumHash.String(),
		a.FirstGroup.Nodes,
		a.SecondGroup.StateHash.SumHash.String(),
		a.SecondGroup.Nodes,
	)
}

func (a *StateHashAlert) Time() time.Time {
	return time.Unix(a.Timestamp, 0)
}

func (a *StateHashAlert) String() string {
	return fmt.Sprintf("%s: %s", a.Type(), a.Message())
}

func (a *StateHashAlert) ID() string {
	var buff bytes.Buffer
	buff.WriteString(a.Type())

	if a.LastCommonStateHashExist {
		buff.WriteString(strconv.Itoa(a.LastCommonStateHashHeight))
		buff.WriteString(a.LastCommonStateHash.SumHash.String())
	}

	for _, node := range a.FirstGroup.Nodes {
		buff.WriteString(node)
	}
	for _, node := range a.SecondGroup.Nodes {
		buff.WriteString(node)
	}

	digest := crypto.MustFastHash(buff.Bytes())
	return digest.String()
}

type AlertFixed struct {
	Timestamp int64 `json:"timestamp"`
	Fixed     Alert `json:"fixed"`
}

func (a *AlertFixed) Type() string {
	return AlertFixedNotificationType
}

func (a *AlertFixed) ID() string {
	digest := crypto.MustFastHash([]byte(a.Type() + a.Fixed.ID()))
	return digest.String()
}

func (a *AlertFixed) Message() string {
	return fmt.Sprintf("Alert has been FIXED: %s", a.Fixed.Message())
}

func (a *AlertFixed) Time() time.Time {
	return time.Unix(a.Timestamp, 0)
}

func (a *AlertFixed) String() string {
	return fmt.Sprintf("%s: %s", a.Type(), a.Message())
}
