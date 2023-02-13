package entities

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFixedAlertUnmarshal(t *testing.T) {
	internalAlert := UnreachableAlert{
		Timestamp: 2,
		Node:      "node1",
	}

	expectedFixedAlert := AlertFixed{
		Timestamp:      1,
		FixedAlertType: UnreachableAlertType,
		Fixed:          &internalAlert,
	}

	data, err := json.Marshal(expectedFixedAlert)
	require.NoError(t, err)

	var fixedAlert AlertFixed
	err = json.Unmarshal(data, &fixedAlert)
	require.NoError(t, err)
	require.Equal(t, expectedFixedAlert, fixedAlert)
}
