package entities

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
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

	// l2 related.
	L2StuckAlertType
)

func GetAllAlertTypesAndNames() map[AlertType]AlertName {
	return map[AlertType]AlertName{
		SimpleAlertType:        SimpleAlertName,
		UnreachableAlertType:   UnreachableAlertName,
		IncompleteAlertType:    IncompleteAlertName,
		InvalidHeightAlertType: InvalidHeightAlertName,
		HeightAlertType:        HeightAlertName,
		StateHashAlertType:     StateHashAlertName,
		AlertFixedType:         AlertFixedName,
		BaseTargetAlertType:    BaseTargetAlertName,
		InternalErrorAlertType: InternalErrorName,
		L2StuckAlertType:       L2StuckAlertName,
	}
}

func (t AlertType) AlertName() (AlertName, bool) {
	var alertName AlertName
	switch t {
	case SimpleAlertType:
		alertName = SimpleAlertName
	case UnreachableAlertType:
		alertName = UnreachableAlertName
	case IncompleteAlertType:
		alertName = IncompleteAlertName
	case InvalidHeightAlertType:
		alertName = InvalidHeightAlertName
	case HeightAlertType:
		alertName = HeightAlertName
	case StateHashAlertType:
		alertName = StateHashAlertName
	case AlertFixedType:
		alertName = AlertFixedName
	case BaseTargetAlertType:
		alertName = BaseTargetAlertName
	case InternalErrorAlertType:
		alertName = InternalErrorName
	case L2StuckAlertType:
		alertName = L2StuckAlertName
	default:
		return alertName, false
	}
	return alertName, true
}

func (t AlertType) Exist() bool {
	_, ok := t.AlertName()
	return ok
}

type AlertName string

const (
	SimpleAlertName        AlertName = "SimpleAlert"
	UnreachableAlertName   AlertName = "UnreachableAlert"
	IncompleteAlertName    AlertName = "IncompleteAlert"
	InvalidHeightAlertName AlertName = "InvalidHeightAlert"
	HeightAlertName        AlertName = "HeightAlert"
	StateHashAlertName     AlertName = "StateHashAlert"
	AlertFixedName         AlertName = "Resolved"
	BaseTargetAlertName    AlertName = "BaseTargetAlert"
	InternalErrorName      AlertName = "InternalErrorAlert"

	// l2 related.
	L2StuckAlertName AlertName = "L2StuckAlert"
)

func (n AlertName) AlertType() (AlertType, bool) {
	var alertType AlertType
	switch n {
	case SimpleAlertName:
		alertType = SimpleAlertType
	case UnreachableAlertName:
		alertType = UnreachableAlertType
	case IncompleteAlertName:
		alertType = IncompleteAlertType
	case InvalidHeightAlertName:
		alertType = InvalidHeightAlertType
	case HeightAlertName:
		alertType = HeightAlertType
	case StateHashAlertName:
		alertType = StateHashAlertType
	case AlertFixedName:
		alertType = AlertFixedType
	case BaseTargetAlertName:
		alertType = BaseTargetAlertType
	case InternalErrorName:
		alertType = InternalErrorAlertType
	case L2StuckAlertName:
		alertType = L2StuckAlertType
	default:
		return alertType, false
	}
	return alertType, true
}

func (n AlertName) String() string { return string(n) }

const (
	InfoLevel  = "Info"
	WarnLevel  = "Warning"
	ErrorLevel = "Error"
)

type Alert interface {
	Name() AlertName
	ID() crypto.Digest // Digest 32 bytes
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

func (a *SimpleAlert) Name() AlertName {
	return SimpleAlertName
}

func (a *SimpleAlert) Message() string {
	return a.Description
}

func (a *SimpleAlert) Time() time.Time {
	return time.Unix(a.Timestamp, 0)
}

func (a *SimpleAlert) ID() crypto.Digest {
	digest := crypto.MustFastHash([]byte(a.Name().String() + a.Description))
	return digest
}

func (a *SimpleAlert) String() string {
	return fmt.Sprintf("%s: %s", a.Name(), a.Message())
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

func (a *UnreachableAlert) Name() AlertName {
	return UnreachableAlertName
}

func (a *UnreachableAlert) Message() string {
	return fmt.Sprintf("Node %s is unreachable", a.Node)
}

func (a *UnreachableAlert) Time() time.Time {
	return time.Unix(a.Timestamp, 0)
}

func (a *UnreachableAlert) ID() crypto.Digest {
	digest := crypto.MustFastHash([]byte(a.Name().String() + a.Node))
	return digest
}

func (a *UnreachableAlert) String() string {
	return fmt.Sprintf("%s: %s", a.Name(), a.Message())
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

func (a *IncompleteAlert) Name() AlertName {
	return IncompleteAlertName
}

func (a *IncompleteAlert) Message() string {
	return fmt.Sprintf("Node %q (%s) has incomplete statement info at height %d", a.Node, a.Version, a.Height)
}

func (a *IncompleteAlert) Time() time.Time {
	return time.Unix(a.Timestamp, 0)
}

func (a *IncompleteAlert) String() string {
	return fmt.Sprintf("%s: %s", a.Name(), a.Message())
}

func (a *IncompleteAlert) ID() crypto.Digest {
	digest := crypto.MustFastHash([]byte(a.Name().String() + a.Node))
	return digest
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

func (a *InvalidHeightAlert) Name() AlertName {
	return InvalidHeightAlertName
}

func (a *InvalidHeightAlert) Message() string {
	return fmt.Sprintf("Node %q (%s) has invalid height %d", a.Node, a.Version, a.Height)
}

func (a *InvalidHeightAlert) Time() time.Time {
	return time.Unix(a.Timestamp, 0)
}

func (a *InvalidHeightAlert) String() string {
	return fmt.Sprintf("%s: %s", a.Name(), a.Message())
}

func (a *InvalidHeightAlert) ID() crypto.Digest {
	digest := crypto.MustFastHash([]byte(a.Name().String() + a.Node))
	return digest
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

func (a *HeightAlert) Name() AlertName {
	return HeightAlertName
}

func (a *HeightAlert) Message() string {
	return fmt.Sprintf(
		"Some node(s) are %d blocks behind\n\nFirst group with height %d:\n%s\n\nSecond group with height %d:\n%s",
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
	return fmt.Sprintf("%s: %s", a.Name(), a.Message())
}

func (a *HeightAlert) ID() crypto.Digest {
	var buff bytes.Buffer
	buff.WriteString(a.Name().String())

	for _, node := range a.OtherHeightGroup.Nodes {
		buff.WriteString(node)
	}

	digest := crypto.MustFastHash(buff.Bytes())
	return digest
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
	Timestamp                 int64 `json:"timestamp"`
	CurrentGroupsBucketHeight int   `json:"current_groups_bucket_height"`
	LastCommonStateHashExist  bool  `json:"last_common_state_hash_exist"`
	// LastCommonStateHashHeight can be empty if LastCommonStateHashExist == false
	LastCommonStateHashHeight int `json:"last_common_state_hash_height"`
	// LastCommonStateHash can be empty if LastCommonStateHashExist == false
	LastCommonStateHash proto.StateHash `json:"last_common_state_hash"`
	FirstGroup          StateHashGroup  `json:"first_group"`
	SecondGroup         StateHashGroup  `json:"second_group"`
}

func (a *StateHashAlert) Name() AlertName {
	return StateHashAlertName
}

func (a *StateHashAlert) Message() string {
	if a.LastCommonStateHashExist {
		return fmt.Sprintf(
			"Nodes have different statehashes at the same height %d\n\n"+
				"First group has\nBlockID: %s\nStateHash: %s\n\n%s\n\n\n"+
				"Second group has\nBlockID: %s\nStateHash: %s\n\n%s\n\n\n"+
				"Fork occured after block %d\nBlock ID: %s\nStatehash: %s",
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
		"Nodes have different statehashes at the same height %d\n\n"+
			"First group has\nBlockID: %s\nStateHash: %s\n\n%s\n\n\n"+
			"Second group has\nBlockID: %s\nStateHash: %s\n\n%s",
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
	return fmt.Sprintf("%s: %s", a.Name(), a.Message())
}

func (a *StateHashAlert) ID() crypto.Digest {
	var buff bytes.Buffer
	buff.WriteString(a.Name().String())

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
	return digest
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

func (a AlertFixed) MarshalJSON() ([]byte, error) {
	type shadowed AlertFixed
	alertFixedWithDescriptor := struct {
		FixedAlertType AlertType `json:"fixed_alert_type"`
		shadowed
	}{a.Fixed.Type(), shadowed(a)}
	return json.Marshal(alertFixedWithDescriptor)
}

func (a *AlertFixed) UnmarshalJSON(msg []byte) error {
	var descriptor struct {
		FixedAlertType AlertType `json:"fixed_alert_type"`
	}
	if err := json.Unmarshal(msg, &descriptor); err != nil {
		return errors.Wrapf(err, "failed to unrmarshal alert type descriptor")
	}
	type shadowed AlertFixed
	var out shadowed
	switch t := descriptor.FixedAlertType; t {
	case SimpleAlertType:
		out.Fixed = &SimpleAlert{}
	case UnreachableAlertType:
		out.Fixed = &UnreachableAlert{}
	case IncompleteAlertType:
		out.Fixed = &IncompleteAlert{}
	case InvalidHeightAlertType:
		out.Fixed = &InvalidHeightAlert{}
	case HeightAlertType:
		out.Fixed = &HeightAlert{}
	case StateHashAlertType:
		out.Fixed = &StateHashAlert{}
	case BaseTargetAlertType:
		out.Fixed = &BaseTargetAlert{}
	case InternalErrorAlertType:
		out.Fixed = &InternalErrorAlert{}
	case L2StuckAlertType:
		out.Fixed = &L2StuckAlert{}
	case AlertFixedType:
		return errors.Errorf("nested fixed alerts (%d) are not allowed", t)
	default:
		return errors.Errorf("failed to unmarshal alert fixed, unknown internal alert type (%d)", t)
	}
	if err := json.Unmarshal(msg, &out); err != nil {
		return errors.Wrapf(err, "failed to unmarshal")
	}
	*a = AlertFixed(out)
	return nil
}

func (a *AlertFixed) Name() AlertName {
	return AlertFixedName
}

func (a *AlertFixed) ID() crypto.Digest {
	digest := crypto.MustFastHash([]byte(a.Name().String() + a.Fixed.ID().String()))
	return digest
}

func (a *AlertFixed) Message() string {
	return fmt.Sprintf("Alert has been fixed: %s", a.Fixed.Message())
}

func (a *AlertFixed) Time() time.Time {
	return time.Unix(a.Timestamp, 0)
}

func (a *AlertFixed) String() string {
	return fmt.Sprintf("%s: %s", a.Name(), a.Message())
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

func (a *BaseTargetAlert) Name() AlertName {
	return BaseTargetAlertName
}

func (a *BaseTargetAlert) ID() crypto.Digest {
	var buff bytes.Buffer
	buff.WriteString(a.Name().String())

	for _, baseTarget := range a.BaseTargetValues {
		buff.WriteString(baseTarget.Node)
	}

	digest := crypto.MustFastHash(buff.Bytes())
	return digest
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
	return fmt.Sprintf("%s: %s", a.Name(), a.Message())
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

func (a *InternalErrorAlert) Name() AlertName {
	return InternalErrorName
}

func (a *InternalErrorAlert) ID() crypto.Digest {
	digest := crypto.MustFastHash([]byte(a.Name().String() + a.Error))
	return digest
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
	return fmt.Sprintf("%s: %s", a.Name(), a.Message())
}

func NewL2StuckAlert(timestamp int64, l2Height uint64, l2Node string) *L2StuckAlert {
	return &L2StuckAlert{
		L2Height:  l2Height,
		L2Node:    l2Node,
		Timestamp: timestamp,
	}
}

type L2StuckAlert struct {
	L2Height  uint64 `json:"l2_height"`
	L2Node    string `json:"l2_node"`
	Timestamp int64  `json:"timestamp"`
}

func (a *L2StuckAlert) Name() AlertName {
	return L2StuckAlertName
}

func (a *L2StuckAlert) Message() string {
	return fmt.Sprintf(
		"Node %s is at the same height for more than 5 minutes", a.L2Node,
	)
}

func (a *L2StuckAlert) Time() time.Time {
	return time.Unix(a.Timestamp, 0)
}

func (a *L2StuckAlert) String() string {
	return fmt.Sprintf("%s: %s", a.Name(), a.Message())
}

func (a *L2StuckAlert) ID() crypto.Digest {
	var buff bytes.Buffer
	buff.WriteString(a.Name().String())
	buff.WriteString(a.L2Node)
	buff.WriteString(strconv.FormatUint(a.L2Height, 10))
	digest := crypto.MustFastHash(buff.Bytes())
	return digest
}

func (a *L2StuckAlert) Type() AlertType {
	return L2StuckAlertType
}

func (a *L2StuckAlert) Level() string {
	return ErrorLevel
}
