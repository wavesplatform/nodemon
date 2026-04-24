package entities_test

import (
	"encoding/hex"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wavesplatform/gowaves/pkg/crypto"
	"github.com/wavesplatform/gowaves/pkg/proto"

	"nodemon/pkg/entities"
)

func TestNodeStatementsSplitByHeight_MinMaxHeight(t *testing.T) {
	tests := []struct {
		split entities.NodeStatementsSplitByHeight
		min   uint64
		max   uint64
	}{
		{
			split: entities.NodeStatementsSplitByHeight{3: {}, 1: {}, 2: {}, 8: {}, 16: {}},
			min:   1,
			max:   16,
		},
		{
			split: entities.NodeStatementsSplitByHeight{53: {}, 7: {}, 234: {}, 42: {}, 86: {}, 44: {}},
			min:   7,
			max:   234,
		},
		{
			split: entities.NodeStatementsSplitByHeight{42: {}},
			min:   42,
			max:   42,
		},
		{
			split: entities.NodeStatementsSplitByHeight{},
			min:   0,
			max:   0,
		},
	}
	for i, test := range tests {
		tcNum := i + 1

		minHeight, maxHeight := test.split.MinMaxHeight()
		require.Equal(t, test.min, minHeight, "test case #%d", tcNum)
		require.Equal(t, test.max, maxHeight, "test case #%d", tcNum)
	}
}

func TestStateHashJSONMatchTags(t *testing.T) {
	id, idErr := proto.NewBlockIDFromBase58("GYdoaG5Urvk9Ud4NiT3hWkkGVwpdsRUoufPgbfrd3hVj")
	require.NoError(t, idErr)
	shBytes, hErr := hex.DecodeString("8dee6d71b683abb926fb51bc5d022ccb98a2dd2e87d97cf08d432b3983e6d2b7")
	require.NoError(t, hErr)
	sh, dErr := crypto.NewDigestFromBytes(shBytes)
	require.NoError(t, dErr)
	const js = `{"blockId":"GYdoaG5Urvk9Ud4NiT3hWkkGVwpdsRUoufPgbfrd3hVj","stateHash":"8dee6d71b683abb926fb51bc5d022ccb98a2dd2e87d97cf08d432b3983e6d2b7"}` //nolint:lll
	esh := entities.StateHash{
		BlockID: id,
		SumHash: sh,
	}
	t.Run("MarshalJSON", func(t *testing.T) {
		res, err := json.Marshal(esh)
		require.NoError(t, err)
		assert.JSONEq(t, js, string(res))
	})
	t.Run("UnmarshalJSON", func(t *testing.T) {
		var tesh entities.StateHash
		err := json.Unmarshal([]byte(js), &tesh)
		require.NoError(t, err)
		assert.Equal(t, esh.BlockID, tesh.BlockID)
		assert.Equal(t, esh.SumHash, tesh.SumHash)
	})
	t.Run("sh1", func(t *testing.T) {
		data, err := json.Marshal(esh)
		require.NoError(t, err)
		var sh1 proto.StateHashV1
		err = json.Unmarshal(data, &sh1)
		require.NoError(t, err)
		assert.Equal(t, esh.SumHash, sh1.GetSumHash())
		assert.Equal(t, esh.BlockID, sh1.GetBlockID())
	})
	t.Run("sh2", func(t *testing.T) {
		data, err := json.Marshal(esh)
		require.NoError(t, err)
		var sh2 proto.StateHashV2
		err = json.Unmarshal(data, &sh2)
		require.NoError(t, err)
		assert.Equal(t, esh.SumHash, sh2.GetSumHash())
		assert.Equal(t, esh.BlockID, sh2.GetBlockID())
	})
}
