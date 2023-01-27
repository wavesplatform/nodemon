package common

import (
	"encoding/binary"
	"testing"

	"github.com/wavesplatform/gowaves/pkg/crypto"
	"github.com/wavesplatform/gowaves/pkg/proto"
	"nodemon/pkg/entities"
)

var formats = []ExpectedExtension{Html, Markdown}

func TestBaseTargetTemplate(t *testing.T) {
	data := &entities.BaseTargetAlert{
		Timestamp: 100,
		BaseTargetValues: []entities.BaseTargetValue{
			{
				Node:       "test1",
				BaseTarget: 150,
			},
			{
				Node:       "test2",
				BaseTarget: 510,
			},
			{
				Node:       "test1",
				BaseTarget: 150,
			},
			{
				Node:       "test2",
				BaseTarget: 510,
			},
			{
				Node:       "test1",
				BaseTarget: 150,
			},
			{
				Node:       "test2",
				BaseTarget: 510,
			},
		},
		Threshold: 101,
	}
	for _, f := range formats {
		_, err := executeTemplate("templates/alerts/base_target_alert", data, f)
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestUnreachableTemplate(t *testing.T) {
	data := &entities.UnreachableAlert{
		Timestamp: 100,
		Node:      "node",
	}
	for _, f := range formats {
		_, err := executeTemplate("templates/alerts/unreachable_alert", data, f)
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestAlertFixed(t *testing.T) {
	unreachable := &entities.UnreachableAlert{
		Timestamp: 100,
		Node:      "blabla",
	}

	data := &entities.AlertFixed{
		Timestamp: 100,
		Fixed:     unreachable,
	}

	fixedStatement := struct {
		PreviousAlert string
	}{
		PreviousAlert: data.Fixed.Message(),
	}
	for _, f := range formats {
		_, err := executeTemplate("templates/alerts/alert_fixed", fixedStatement, f)
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestHeightTemplate(t *testing.T) {
	heightAlert := &entities.HeightAlert{
		Timestamp: 100,
		MaxHeightGroup: entities.HeightGroup{
			Height: 2,
			Nodes:  entities.Nodes{"node 3", "node 4"},
		},
		OtherHeightGroup: entities.HeightGroup{
			Height: 1,
			Nodes:  entities.Nodes{"node 1", "node 2"},
		},
	}

	type group struct {
		Nodes  []string
		Height int
	}

	heightStatement := struct {
		HeightDifference int
		FirstGroup       group
		SecondGroup      group
	}{
		HeightDifference: heightAlert.MaxHeightGroup.Height - heightAlert.OtherHeightGroup.Height,
		FirstGroup: group{
			Nodes:  heightAlert.MaxHeightGroup.Nodes,
			Height: heightAlert.MaxHeightGroup.Height,
		},
		SecondGroup: group{
			Nodes:  heightAlert.OtherHeightGroup.Nodes,
			Height: heightAlert.OtherHeightGroup.Height,
		},
	}
	for _, f := range formats {
		_, err := executeTemplate("templates/alerts/height_alert", heightStatement, f)

		if err != nil {
			t.Fatal(err)
		}
	}
}

type shInfo struct {
	id proto.BlockID
	sh proto.StateHash
}

func sequentialBlockID(i int) proto.BlockID {
	d := crypto.Digest{}
	binary.BigEndian.PutUint64(d[:8], uint64(i))
	return proto.NewBlockIDFromDigest(d)
}

func sequentialStateHash(blockID proto.BlockID, i int) proto.StateHash {
	d := crypto.Digest{}
	binary.BigEndian.PutUint64(d[:8], uint64(i))
	return proto.StateHash{
		BlockID:      blockID,
		SumHash:      d,
		FieldsHashes: proto.FieldsHashes{},
	}
}

func generateStateHashes(o, n int) []shInfo {
	r := make([]shInfo, n)
	for i := 0; i < n; i++ {
		id := sequentialBlockID(o + i + 1)
		sh := sequentialStateHash(id, o+i+101)
		r[i] = shInfo{id: id, sh: sh}
	}
	return r
}

func TestStateHashTemplate(t *testing.T) {

	shInfo := generateStateHashes(1, 5)
	stateHashAlert := &entities.StateHashAlert{
		CurrentGroupsBucketHeight: 100,
		LastCommonStateHashExist:  true,
		LastCommonStateHashHeight: 1,
		LastCommonStateHash:       shInfo[0].sh,
		FirstGroup: entities.StateHashGroup{
			Nodes:     entities.Nodes{"a"},
			StateHash: shInfo[0].sh,
		},
		SecondGroup: entities.StateHashGroup{
			Nodes:     entities.Nodes{"b"},
			StateHash: shInfo[0].sh,
		},
	}

	type stateHashGroup struct {
		BlockID   string
		Nodes     []string
		StateHash string
	}
	stateHashStatement := struct {
		SameHeight               int
		LastCommonStateHashExist bool
		ForkHeight               int
		ForkBlockID              string
		ForkStateHash            string

		FirstGroup  stateHashGroup
		SecondGroup stateHashGroup
	}{
		SameHeight:               stateHashAlert.CurrentGroupsBucketHeight,
		LastCommonStateHashExist: stateHashAlert.LastCommonStateHashExist,
		ForkHeight:               stateHashAlert.LastCommonStateHashHeight,
		ForkBlockID:              stateHashAlert.LastCommonStateHash.BlockID.String(),
		ForkStateHash:            stateHashAlert.LastCommonStateHash.SumHash.Hex(),

		FirstGroup: stateHashGroup{
			BlockID:   stateHashAlert.FirstGroup.StateHash.BlockID.String(),
			Nodes:     stateHashAlert.FirstGroup.Nodes,
			StateHash: stateHashAlert.FirstGroup.StateHash.SumHash.Hex(),
		},
		SecondGroup: stateHashGroup{
			BlockID:   stateHashAlert.SecondGroup.StateHash.BlockID.String(),
			Nodes:     stateHashAlert.SecondGroup.Nodes,
			StateHash: stateHashAlert.SecondGroup.StateHash.SumHash.Hex(),
		},
	}
	for _, f := range formats {
		_, err := executeTemplate("templates/alerts/state_hash_alert", stateHashStatement, f)
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestIncompleteTemplate(t *testing.T) {
	data := &entities.IncompleteAlert{
		NodeStatement: entities.NodeStatement{Node: "a", Version: "1", Height: 1},
	}
	for _, f := range formats {
		_, err := executeTemplate("templates/alerts/incomplete_alert", data, f)
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestInternalErrorTemplate(t *testing.T) {
	data := &entities.InternalErrorAlert{
		Error: "error",
	}
	for _, f := range formats {
		_, err := executeTemplate("templates/alerts/internal_error_alert", data, f)
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestInvalidHeightTemplate(t *testing.T) {
	data := &entities.InvalidHeightAlert{
		NodeStatement: entities.NodeStatement{Node: "a", Version: "1", Height: 1},
	}
	for _, f := range formats {
		_, err := executeTemplate("templates/alerts/invalid_height_alert", data, f)
		if err != nil {
			t.Fatal(err)
		}
	}
}
