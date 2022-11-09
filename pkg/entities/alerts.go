package entities

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
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
	BaseTargetAlertType
	InternalErrorAlertType
)

var AlertTypes = map[AlertType]string{
	SimpleAlertType:        SimpleAlertNotification,
	UnreachableAlertType:   UnreachableAlertNotification,
	IncompleteAlertType:    IncompleteAlertNotification,
	InvalidHeightAlertType: InvalidHeightAlertNotification,
	HeightAlertType:        HeightAlertNotification,
	StateHashAlertType:     StateHashAlertNotification,
	AlertFixedType:         AlertFixedNotification,
	BaseTargetAlertType:    BaseTargetAlertNotification,
	InternalErrorAlertType: InternalErrorNotification,
}

const (
	SimpleAlertNotification        = "SimpleAlert"
	UnreachableAlertNotification   = "UnreachableAlert"
	IncompleteAlertNotification    = "IncompleteAlert"
	InvalidHeightAlertNotification = "InvalidHeightAlert"
	HeightAlertNotification        = "HeightAlert"
	StateHashAlertNotification     = "StateHashAlert"
	AlertFixedNotification         = "AlertFixed"
	BaseTargetAlertNotification    = "BaseTargetAlert"
	InternalErrorNotification      = "InternalErrorAlert"
)

const (
	InfoLevel  = "Info"
	WarnLevel  = "Warning"
	ErrorLevel = "Error"
)

type Alert interface {
	Notification
	ID() string
	Message() string
	Time() time.Time
	Type() AlertType
	Level() string
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

func (a *SimpleAlert) Level() string {
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
	return fmt.Sprintf("Node %s is unreachable", a.Node)
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

func (a *UnreachableAlert) Level() string {
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

func (a *IncompleteAlert) Level() string {
	return ErrorLevel
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

func (a *InvalidHeightAlert) Level() string {
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
	return fmt.Sprintf("Some node(s) are %d blocks behind\n\nFirst group with height %d:\n%s\n\nSecond group with height %d:\n%s",
		a.MaxHeightGroup.Height-a.OtherHeightGroup.Height,
		a.MaxHeightGroup.Height,
		strings.Join(a.MaxHeightGroup.Nodes, "\n"),
		a.OtherHeightGroup.Height,
		strings.Join(a.OtherHeightGroup.Nodes, "\n"),
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

	for _, node := range a.OtherHeightGroup.Nodes {
		buff.WriteString(node)
	}

	digest := crypto.MustFastHash(buff.Bytes())
	return digest.String()
}

func (a *HeightAlert) Type() AlertType {
	return HeightAlertType
}

func (a *HeightAlert) Level() string {
	return ErrorLevel
}

type StateHashGroup struct {
	Nodes     Nodes           `json:"nodes"`
	StateHash proto.StateHash `json:"state_hash"`
}

type StateHashAlert struct {
	Timestamp                 int64           `json:"timestamp"`
	CurrentGroupsBucketHeight int             `json:"current_groups_bucket_height"`
	LastCommonStateHashExist  bool            `json:"last_common_state_hash_exist"`
	LastCommonStateHashHeight int             `json:"last_common_state_hash_height"` // can be empty if LastCommonStateHashExist == false
	LastCommonStateHash       proto.StateHash `json:"last_common_state_hash"`        // can be empty if LastCommonStateHashExist == false
	FirstGroup                StateHashGroup  `json:"first_group"`
	SecondGroup               StateHashGroup  `json:"second_group"`
}

func (a *StateHashAlert) ShortDescription() string {
	return StateHashAlertNotification
}

func (a *StateHashAlert) Message() string {
	if a.LastCommonStateHashExist {
		return fmt.Sprintf(
			"Nodes have different statehashes at the same height %d\n\nFirst group has\nBlockID: %s\nStateHash: %s\n\n%s\n\n\nSecond group has\nBlockID: %s\nStateHash: %s\n\n%s\n\n\nFork occured after block %d\nBlock ID: %s\nStatehash: %s",
			a.CurrentGroupsBucketHeight,
			a.FirstGroup.StateHash.BlockID.String(),
			a.FirstGroup.StateHash.SumHash.Hex(),
			strings.Join(a.FirstGroup.Nodes, "\n"),
			a.SecondGroup.StateHash.BlockID.String(),
			a.SecondGroup.StateHash.SumHash.Hex(),
			strings.Join(a.SecondGroup.Nodes, "\n"),
			a.LastCommonStateHashHeight,
			a.LastCommonStateHash.BlockID.String(),
			a.LastCommonStateHash.SumHash.Hex(),
		)
	}
	return fmt.Sprintf(
		"Nodes have different statehashes at the same height %d\n\nFirst group has\nBlockID: %s\nStateHash: %s\n\n%s\n\n\nSecond group has\nBlockID: %s\nStateHash: %s\n\n%s",
		a.CurrentGroupsBucketHeight,
		a.FirstGroup.StateHash.BlockID.String(),
		a.FirstGroup.StateHash.SumHash.Hex(),
		strings.Join(a.FirstGroup.Nodes, "\n"),
		a.SecondGroup.StateHash.BlockID.String(),
		a.SecondGroup.StateHash.SumHash.Hex(),
		strings.Join(a.SecondGroup.Nodes, "\n"),
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

func (a *StateHashAlert) Level() string {
	return ErrorLevel
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
	return fmt.Sprintf("Alert has been fixed: %s", a.Fixed.Message())
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

func (a *AlertFixed) Level() string {
	return InfoLevel
}

type BaseTargetValue struct {
	Node       string `json:"node"`
	BaseTarget int    `json:"base_target"`
}

type BaseTargetAlert struct {
	Timestamp        int64             `json:"timestamp"`
	BaseTargetValues []BaseTargetValue `json:"thresh_holds"`
	Threshold        int               `json:"default_value"`
}

func (a *BaseTargetAlert) ShortDescription() string {
	return BaseTargetAlertNotification
}

func (a *BaseTargetAlert) ID() string {
	var buff bytes.Buffer
	buff.WriteString(a.ShortDescription())

	for _, baseTarget := range a.BaseTargetValues {
		buff.WriteString(baseTarget.Node)
	}

	digest := crypto.MustFastHash(buff.Bytes())
	return digest.String()
}

func (a *BaseTargetAlert) Message() string {
	msg := fmt.Sprintf("Base target is greater than the treshold value. The treshold value is %d\n\n", a.Threshold)
	for _, baseTarget := range a.BaseTargetValues {
		msg += fmt.Sprintf("Node %s\nBase target: %d\n\n", baseTarget.Node, baseTarget.BaseTarget)
	}

	return msg
}

func (a *BaseTargetAlert) Time() time.Time {
	return time.Unix(a.Timestamp, 0)
}

func (a *BaseTargetAlert) String() string {
	return fmt.Sprintf("%s: %s", a.ShortDescription(), a.Message())
}

func (a *BaseTargetAlert) Type() AlertType {
	return BaseTargetAlertType
}

func (a *BaseTargetAlert) Level() string {
	return ErrorLevel
}

type InternalErrorAlert struct {
	Timestamp int64  `json:"timestamp"`
	Error     string `json:"error"`
}

func NewInternalErrorAlert(timestamp int64, err error) *InternalErrorAlert {
	return &InternalErrorAlert{
		Timestamp: timestamp,
		Error:     fmt.Sprintf("%v", err),
	}
}

func (a *InternalErrorAlert) ShortDescription() string {
	return InternalErrorNotification
}

func (a *InternalErrorAlert) ID() string {
	digest := crypto.MustFastHash([]byte(a.ShortDescription() + a.Error))
	return digest.String()
}

func (a *InternalErrorAlert) Message() string {
	return fmt.Sprintf("An internal error has occurred: %s", a.Error)
}

func (a *InternalErrorAlert) Time() time.Time {
	return time.Unix(a.Timestamp, 0)
}

func (a *InternalErrorAlert) Type() AlertType {
	return InternalErrorAlertType
}

func (a *InternalErrorAlert) Level() string {
	return WarnLevel
}

func (a *InternalErrorAlert) String() string {
	return fmt.Sprintf("%s: %s", a.ShortDescription(), a.Message())
}
