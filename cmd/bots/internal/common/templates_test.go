package common

import (
	"encoding/binary"
	"flag"
	"os"
	"path/filepath"
	"testing"

	commonMessages "nodemon/cmd/bots/internal/common/messages"
	"nodemon/pkg/entities"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wavesplatform/gowaves/pkg/crypto"
	"github.com/wavesplatform/gowaves/pkg/proto"
)

var (
	update = flag.Bool("update", false, "update the golden files of this test")
)

func expectedFormats() []ExpectedExtension {
	return []ExpectedExtension{HTML, Markdown}
}

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

func TestSimpleAlertTemplate(t *testing.T) {
	data := entities.SimpleAlert{
		Timestamp:   100500,
		Description: "Simple alert !!!",
	}
	for _, f := range expectedFormats() {
		const template = "templates/alerts/simple_alert"
		actual, err := executeTemplate(template, data, f)
		require.NoError(t, err)
		expected := goldenValue(t, template, f, actual)
		assert.Equal(t, expected, actual)
	}
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
	for _, f := range expectedFormats() {
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
	for _, f := range expectedFormats() {
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
		Fixed: unreachable,
	}

	statement := fixedStatement{
		PreviousAlert: data.Fixed.ShortDescription(),
	}
	for _, f := range expectedFormats() {
		const template = "templates/alerts/alert_fixed"
		actual, err := executeTemplate(template, statement, f)
		require.NoError(t, err)
		expected := goldenValue(t, template, f, actual)
		assert.Equal(t, expected, actual)
	}
}

func TestHeightTemplate(t *testing.T) {
	heightAlert := &entities.HeightAlert{
		MaxHeightGroup: entities.HeightGroup{
			Height: 2,
			Nodes:  entities.Nodes{"node 3", "node 4"},
		},
		OtherHeightGroup: entities.HeightGroup{
			Height: 1,
			Nodes:  entities.Nodes{"node 1", "node 2"},
		},
	}

	statement := heightStatement{
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
	for _, f := range expectedFormats() {
		const template = "templates/alerts/height_alert"
		actual, err := executeTemplate(template, statement, f)
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
	stateHashInfo := generateStateHashes(1, 5)
	stateHashAlert := &entities.StateHashAlert{
		CurrentGroupsBucketHeight: 100,
		LastCommonStateHashExist:  true,
		LastCommonStateHashHeight: 1,
		LastCommonStateHash:       stateHashInfo[0].sh,
		FirstGroup: entities.StateHashGroup{
			Nodes:     entities.Nodes{"a"},
			StateHash: stateHashInfo[0].sh,
		},
		SecondGroup: entities.StateHashGroup{
			Nodes:     entities.Nodes{"b"},
			StateHash: stateHashInfo[0].sh,
		},
	}

	statement := stateHashStatement{
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
	for _, f := range expectedFormats() {
		const template = "templates/alerts/state_hash_alert"
		actual, err := executeTemplate(template, statement, f)
		require.NoError(t, err)
		expected := goldenValue(t, template, f, actual)
		assert.Equal(t, expected, actual)
	}
}

func TestIncompleteTemplate(t *testing.T) {
	data := &entities.IncompleteAlert{
		NodeStatement: entities.NodeStatement{Node: "a", Version: "1", Height: 1},
	}
	for _, f := range expectedFormats() {
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
	for _, f := range expectedFormats() {
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
	for _, f := range expectedFormats() {
		const template = "templates/alerts/invalid_height_alert"
		actual, err := executeTemplate(template, data, f)
		require.NoError(t, err)
		expected := goldenValue(t, template, f, actual)
		assert.Equal(t, expected, actual)
	}
}

func TestNodesListTemplateHTML(t *testing.T) {
	data := []entities.Node{
		{URL: "blah", Enabled: false, Alias: "al"},
		{URL: "lala", Enabled: true, Alias: "la"},
		{URL: "b", Enabled: true, Alias: ""},
		{URL: "m", Enabled: true, Alias: ""},
	}
	const (
		template = "templates/nodes_list"
		ext      = HTML
	)
	urls := nodesToUrls(data)
	actual, err := executeTemplate(template, urls, ext)
	require.NoError(t, err)
	expected := goldenValue(t, template, ext, actual)
	assert.Equal(t, expected, actual)
}

func TestNodeStatementTemplateHTML(t *testing.T) {
	data := nodeStatement{
		Node:      "blah-blah",
		Height:    999,
		Timestamp: 1000500,
		StateHash: "sample-state-hash",
		Version:   "v1.0.1",
	}
	const (
		template = "templates/node_statement"
		ext      = HTML
	)
	actual, err := executeTemplate(template, data, ext)
	require.NoError(t, err)
	expected := goldenValue(t, template, ext, actual)
	assert.Equal(t, expected, actual)
}

func TestSubscriptionsTemplateHTML(t *testing.T) {
	data := subscriptionsList{
		SubscribedTo:     []subscribed{{AlertName: "SubscribedToFirst"}, {AlertName: "SubscribedToSecond"}},
		UnsubscribedFrom: []unsubscribed{{AlertName: "UnsubscribedFromFirst"}, {AlertName: "UnsubscribedFromSecond"}},
	}
	const (
		template = "templates/subscriptions"
		ext      = HTML
	)
	actual, err := executeTemplate(template, data, ext)
	require.NoError(t, err)
	expected := goldenValue(t, template, ext, actual)
	assert.Equal(t, expected, actual)
}

func TestNodesStatusDifferentHashesTemplate(t *testing.T) {
	data := []NodeStatus{
		{
			URL:     "some-url",
			Sumhash: "some-sum-hash",
			Status:  "some-status",
			Height:  "1234",
			BlockID: "some-block-id",
		},
		{
			URL:     "another-url",
			Sumhash: "another-sum-hash",
			Status:  "some-status",
			Height:  "4321",
			BlockID: "another-block-id",
		},
		{
			URL:     "one-more-url",
			Sumhash: "one-more-sum-hash",
			Status:  "one-more-status",
			Height:  "9876543245",
			BlockID: "one-more-block-id",
		},
	}
	for _, f := range expectedFormats() {
		const template = "templates/nodes_status_different_hashes"
		actual, err := executeTemplate(template, data, f)
		require.NoError(t, err)
		expected := goldenValue(t, template, f, actual)
		assert.Equal(t, expected, actual)
	}
}

func TestNodesStatusDifferentHeightsTemplate(t *testing.T) {
	data := []NodeStatus{
		{
			URL:     "some-url",
			Sumhash: "some-sum-hash",
			Status:  "some-status",
			Height:  "1234",
			BlockID: "some-block-id",
		},
		{
			URL:     "another-url",
			Sumhash: "another-sum-hash",
			Status:  "some-status",
			Height:  "4321",
			BlockID: "another-block-id",
		},
		{
			URL:     "one-more-url",
			Sumhash: "one-more-sum-hash",
			Status:  "one-more-status",
			Height:  "9876543245",
			BlockID: "one-more-block-id",
		},
	}
	for _, f := range expectedFormats() {
		const template = "templates/nodes_status_different_heights"
		actual, err := executeTemplate(template, data, f)
		require.NoError(t, err)
		expected := goldenValue(t, template, f, actual)
		assert.Equal(t, expected, actual)
	}
}

func TestNodesStatusOkTemplate(t *testing.T) {
	data := []NodeStatus{
		{
			URL:     "some-url",
			Sumhash: "some-sum-hash",
			Status:  "some-status",
			Height:  "1234",
			BlockID: "some-block-id",
		},
		{
			URL:     "another-url",
			Sumhash: "another-sum-hash",
			Status:  "some-status",
			Height:  "4321",
			BlockID: "another-block-id",
		},
		{
			URL:     "one-more-url",
			Sumhash: "one-more-sum-hash",
			Status:  "one-more-status",
			Height:  "9876543245",
			BlockID: "one-more-block-id",
		},
	}
	for _, f := range expectedFormats() {
		const template = "templates/nodes_status_ok"
		actual, err := executeTemplate(template, data, f)
		require.NoError(t, err)
		expected := goldenValue(t, template, f, actual)
		assert.Equal(t, expected, actual)
	}
}

func TestNodesStatusOkShortTemplate(t *testing.T) {
	data := shortOkNodes{
		TimeEmoji:   commonMessages.TimerMsg,
		NodesNumber: 101,
		Height:      "31425 (this_should_be_a_string_probably)",
	}
	for _, f := range expectedFormats() {
		const template = "templates/nodes_status_ok_short"
		actual, err := executeTemplate(template, data, f)
		require.NoError(t, err)
		expected := goldenValue(t, template, f, actual)
		assert.Equal(t, expected, actual)
	}
}

func TestNodesStatusUnavailableTemplate(t *testing.T) {
	data := []NodeStatus{
		{
			URL:     "some-url",
			Sumhash: "some-sum-hash",
			Status:  "some-status",
			Height:  "1234",
			BlockID: "some-block-id",
		},
		{
			URL:     "another-url",
			Sumhash: "another-sum-hash",
			Status:  "some-status",
			Height:  "4321",
			BlockID: "another-block-id",
		},
		{
			URL:     "one-more-url",
			Sumhash: "one-more-sum-hash",
			Status:  "one-more-status",
			Height:  "9876543245",
			BlockID: "one-more-block-id",
		},
	}
	for _, f := range expectedFormats() {
		const template = "templates/nodes_status_unavailable"
		actual, err := executeTemplate(template, data, f)
		require.NoError(t, err)
		expected := goldenValue(t, template, f, actual)
		assert.Equal(t, expected, actual)
	}
}
