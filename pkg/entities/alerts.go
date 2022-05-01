package entities

import (
	"bytes"
	"fmt"
	"strconv"
	"time"

	"github.com/wavesplatform/gowaves/pkg/crypto"
	"github.com/wavesplatform/gowaves/pkg/proto"
)

type AlertType byte

const (
	SimpleAlertType AlertType = iota + 1
	UnreachableAlertType
	IncompleteAlertType
	InvalidHeightAlertType
	HeightAlertType
	StateHashAlertType
	AlertFixedType
)

var AlertTypes = map[AlertType]string{
	SimpleAlertType:        SimpleAlertNotification,
	UnreachableAlertType:   UnreachableAlertNotification,
	IncompleteAlertType:    IncompleteAlertNotification,
	InvalidHeightAlertType: InvalidHeightAlertNotification,
	HeightAlertType:        HeightAlertNotification,
	StateHashAlertType:     StateHashAlertNotification,
	AlertFixedType:         AlertFixedNotification,
}

const (
	SimpleAlertNotification        = "SimpleAlert"
	UnreachableAlertNotification   = "UnreachableAlert"
	IncompleteAlertNotification    = "IncompleteAlert"
	InvalidHeightAlertNotification = "InvalidHeightAlert"
	HeightAlertNotification        = "HeightAlert"
	StateHashAlertNotification     = "StateHashAlert"
	AlertFixedNotification         = "AlertFixed"
)

const (
	InfoLevel     = "Info"
	ErrorLevel    = "Error"
	criticalLevel = "Critical"
)

type Alert interface {
	Notification
	ID() string
	Message() string
	Time() time.Time
	Type() AlertType
	Severity() string
	fmt.Stringer
}

type SimpleAlert struct {
	Timestamp   int64  `json:"timestamp"`
	Description string `json:"description"`
}

func (a *SimpleAlert) ShortDescription() string {
	return SimpleAlertNotification
}

func (a *SimpleAlert) Message() string {
	return a.Description
}

func (a *SimpleAlert) Time() time.Time {
	return time.Unix(a.Timestamp, 0)
}

func (a *SimpleAlert) ID() string {
	digest := crypto.MustFastHash([]byte(a.ShortDescription() + a.Description))
	return digest.String()
}

func (a *SimpleAlert) String() string {
	return fmt.Sprintf("%s: %s", a.ShortDescription(), a.Message())
}

func (a *SimpleAlert) Type() AlertType {
	return SimpleAlertType
}

func (a *SimpleAlert) Severity() string {
	return InfoLevel
}

type UnreachableAlert struct {
	Timestamp int64  `json:"timestamp"`
	Node      string `json:"node"`
}

func (a *UnreachableAlert) ShortDescription() string {
	return UnreachableAlertNotification
}

func (a *UnreachableAlert) Message() string {
	return fmt.Sprintf("Node %q is UNREACHABLE", a.Node)
}

func (a *UnreachableAlert) Time() time.Time {
	return time.Unix(a.Timestamp, 0)
}

func (a *UnreachableAlert) ID() string {
	digest := crypto.MustFastHash([]byte(a.ShortDescription() + a.Node))
	return digest.String()
}

func (a *UnreachableAlert) String() string {
	return fmt.Sprintf("%s: %s", a.ShortDescription(), a.Message())
}

func (a *UnreachableAlert) Type() AlertType {
	return UnreachableAlertType
}

func (a *UnreachableAlert) Severity() string {
	return ErrorLevel
}

type IncompleteAlert struct {
	NodeStatement
}

func (a *IncompleteAlert) ShortDescription() string {
	return IncompleteAlertNotification
}

func (a *IncompleteAlert) Message() string {
	return fmt.Sprintf("Node %q (%s) has incomplete statement info at height %d", a.Node, a.Version, a.Height)
}

func (a *IncompleteAlert) Time() time.Time {
	return time.Unix(a.Timestamp, 0)
}

func (a *IncompleteAlert) String() string {
	return fmt.Sprintf("%s: %s", a.ShortDescription(), a.Message())
}

func (a *IncompleteAlert) ID() string {
	digest := crypto.MustFastHash([]byte(a.ShortDescription() + a.Node))
	return digest.String()
}

func (a *IncompleteAlert) Type() AlertType {
	return IncompleteAlertType
}

func (a *IncompleteAlert) Severity() string {
	return InfoLevel
}

type InvalidHeightAlert struct {
	NodeStatement
}

func (a *InvalidHeightAlert) ShortDescription() string {
	return InvalidHeightAlertNotification
}

func (a *InvalidHeightAlert) Message() string {
	return fmt.Sprintf("Node %q (%s) has invalid height %d", a.Node, a.Version, a.Height)
}

func (a *InvalidHeightAlert) Time() time.Time {
	return time.Unix(a.Timestamp, 0)
}

func (a *InvalidHeightAlert) String() string {
	return fmt.Sprintf("%s: %s", a.ShortDescription(), a.Message())
}

func (a *InvalidHeightAlert) ID() string {
	digest := crypto.MustFastHash([]byte(a.ShortDescription() + a.Node))
	return digest.String()
}

func (a *InvalidHeightAlert) Type() AlertType {
	return InvalidHeightAlertType
}

func (a *InvalidHeightAlert) Severity() string {
	return ErrorLevel
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

func (a *HeightAlert) ShortDescription() string {
	return HeightAlertNotification
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
	return fmt.Sprintf("%s: %s", a.ShortDescription(), a.Message())
}

func (a *HeightAlert) ID() string {
	var buff bytes.Buffer
	buff.WriteString(a.ShortDescription())

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

func (a *HeightAlert) Type() AlertType {
	return HeightAlertType
}

func (a *HeightAlert) Severity() string {
	return ErrorLevel
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

func (a *StateHashAlert) ShortDescription() string {
	return StateHashAlertNotification
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
	return fmt.Sprintf("%s: %s", a.ShortDescription(), a.Message())
}

func (a *StateHashAlert) ID() string {
	var buff bytes.Buffer
	buff.WriteString(a.ShortDescription())

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

func (a *StateHashAlert) Type() AlertType {
	return StateHashAlertType
}

func (a *StateHashAlert) Severity() string {
	return InfoLevel
}

type AlertFixed struct {
	Timestamp int64 `json:"timestamp"`
	Fixed     Alert `json:"fixed"`
}

func (a *AlertFixed) ShortDescription() string {
	return AlertFixedNotification
}

func (a *AlertFixed) ID() string {
	digest := crypto.MustFastHash([]byte(a.ShortDescription() + a.Fixed.ID()))
	return digest.String()
}

func (a *AlertFixed) Message() string {
	return fmt.Sprintf("Alert has been FIXED: %s", a.Fixed.Message())
}

func (a *AlertFixed) Time() time.Time {
	return time.Unix(a.Timestamp, 0)
}

func (a *AlertFixed) String() string {
	return fmt.Sprintf("%s: %s", a.ShortDescription(), a.Message())
}

func (a *AlertFixed) Type() AlertType {
	return AlertFixedType
}

func (a *AlertFixed) Severity() string {
	return InfoLevel
}
