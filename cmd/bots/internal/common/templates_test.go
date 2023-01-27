package common

import (
	"encoding/binary"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wavesplatform/gowaves/pkg/crypto"
	"github.com/wavesplatform/gowaves/pkg/proto"
	"nodemon/pkg/entities"
)

var formats = []ExpectedExtension{Html, Markdown}

var (
	update = flag.Bool("update", false, "update the golden files of this test")
)

func TestMain(m *testing.M) {
	flag.Parse()
	os.Exit(m.Run())
}

func goldenValue(t *testing.T, goldenFile string, expectedExtension ExpectedExtension, actual string) string {
	t.Helper()
	goldenPath := filepath.Join("testdata", goldenFile+string(expectedExtension)+".golden")

	if *update {
		dir := filepath.Dir(goldenPath)
		err := os.MkdirAll(dir, 0755)
		require.NoError(t, err, "Error creating directories tree %s", dir)
		err = os.WriteFile(goldenPath, []byte(actual), 0644)
		require.NoError(t, err, "Error writing to file %s", goldenPath)
		return actual
	}

	content, err := os.ReadFile(goldenPath)
	require.NoError(t, err, "Error opening file %s", goldenPath)
	return string(content)
}

func TestBaseTargetTemplate(t *testing.T) {
	data := &entities.BaseTargetAlert{
		Timestamp: 100,
		BaseTargetValues: []entities.BaseTargetValue{
			{Node: "test1", BaseTarget: 150},
			{Node: "test2", BaseTarget: 510},
		},
		Threshold: 101,
	}
	for _, f := range formats {
		const template = "templates/alerts/base_target_alert"
		actual, err := executeTemplate(template, data, f)
		require.NoError(t, err)
		expected := goldenValue(t, template, f, actual)
		assert.Equal(t, expected, actual)
	}
}

func TestUnreachableTemplate(t *testing.T) {
	data := &entities.UnreachableAlert{
		Timestamp: 100,
		Node:      "node",
	}
	for _, f := range formats {
		const template = "templates/alerts/unreachable_alert"
		actual, err := executeTemplate(template, data, f)
		require.NoError(t, err)
		expected := goldenValue(t, template, f, actual)
		assert.Equal(t, expected, actual)
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

	fixedStatement := fixedStatement{
		PreviousAlert: data.Fixed.Message(),
	}
	for _, f := range formats {
		const template = "templates/alerts/alert_fixed"
		actual, err := executeTemplate(template, fixedStatement, f)
		require.NoError(t, err)
		expected := goldenValue(t, template, f, actual)
		assert.Equal(t, expected, actual)
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

	heightStatement := heightStatement{
		HeightDifference: heightAlert.MaxHeightGroup.Height - heightAlert.OtherHeightGroup.Height,
		FirstGroup: heightStatementGroup{
			Nodes:  heightAlert.MaxHeightGroup.Nodes,
			Height: heightAlert.MaxHeightGroup.Height,
		},
		SecondGroup: heightStatementGroup{
			Nodes:  heightAlert.OtherHeightGroup.Nodes,
			Height: heightAlert.OtherHeightGroup.Height,
		},
	}
	for _, f := range formats {
		const template = "templates/alerts/height_alert"
		actual, err := executeTemplate(template, heightStatement, f)
		require.NoError(t, err)
		expected := goldenValue(t, template, f, actual)
		assert.Equal(t, expected, actual)
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

	stateHashStatement := stateHashStatement{
		SameHeight:               stateHashAlert.CurrentGroupsBucketHeight,
		LastCommonStateHashExist: stateHashAlert.LastCommonStateHashExist,
		ForkHeight:               stateHashAlert.LastCommonStateHashHeight,
		ForkBlockID:              stateHashAlert.LastCommonStateHash.BlockID.String(),
		ForkStateHash:            stateHashAlert.LastCommonStateHash.SumHash.Hex(),

		FirstGroup: stateHashStatementGroup{
			BlockID:   stateHashAlert.FirstGroup.StateHash.BlockID.String(),
			Nodes:     stateHashAlert.FirstGroup.Nodes,
			StateHash: stateHashAlert.FirstGroup.StateHash.SumHash.Hex(),
		},
		SecondGroup: stateHashStatementGroup{
			BlockID:   stateHashAlert.SecondGroup.StateHash.BlockID.String(),
			Nodes:     stateHashAlert.SecondGroup.Nodes,
			StateHash: stateHashAlert.SecondGroup.StateHash.SumHash.Hex(),
		},
	}
	for _, f := range formats {
		const template = "templates/alerts/state_hash_alert"
		actual, err := executeTemplate(template, stateHashStatement, f)
		require.NoError(t, err)
		expected := goldenValue(t, template, f, actual)
		assert.Equal(t, expected, actual)
	}
}

func TestIncompleteTemplate(t *testing.T) {
	data := &entities.IncompleteAlert{
		NodeStatement: entities.NodeStatement{Node: "a", Version: "1", Height: 1},
	}
	for _, f := range formats {
		const template = "templates/alerts/incomplete_alert"
		actual, err := executeTemplate(template, data, f)
		require.NoError(t, err)
		expected := goldenValue(t, template, f, actual)
		assert.Equal(t, expected, actual)
	}
}

func TestInternalErrorTemplate(t *testing.T) {
	data := &entities.InternalErrorAlert{
		Error: "error",
	}
	for _, f := range formats {
		const template = "templates/alerts/internal_error_alert"
		actual, err := executeTemplate(template, data, f)
		require.NoError(t, err)
		expected := goldenValue(t, template, f, actual)
		assert.Equal(t, expected, actual)
	}
}

func TestInvalidHeightTemplate(t *testing.T) {
	data := &entities.InvalidHeightAlert{
		NodeStatement: entities.NodeStatement{Node: "a", Version: "1", Height: 1},
	}
	for _, f := range formats {
		const template = "templates/alerts/invalid_height_alert"
		actual, err := executeTemplate(template, data, f)
		require.NoError(t, err)
		expected := goldenValue(t, template, f, actual)
		assert.Equal(t, expected, actual)
	}
}
