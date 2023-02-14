package entities

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFixedAlertJSON(t *testing.T) {
	t.Run("ShadowedTypeDoesNotImplementUnmarshaler", func(t *testing.T) {
		type shadowed AlertFixed
		var v shadowed // see trick in AlertFixed.UnmarshalJSON
		_, typeImplements := interface{}(v).(json.Unmarshaler)
		require.False(t, typeImplements, "type must not implement Unmarshaler")
		_, pointerImplements := interface{}(&v).(json.Unmarshaler)
		require.False(t, pointerImplements, "pointer must not implement Unmarshaler")
	})
	var (
		expectedFixedAlert = AlertFixed{
			Timestamp:      1,
			FixedAlertType: UnreachableAlertType,
			Fixed: &UnreachableAlert{
				Timestamp: 2,
				Node:      "node1",
			},
		}
		expectedJSON = `{"timestamp":1,"fixed":{"timestamp":2,"node":"node1"},"fixed_alert_type":2}`
	)
	t.Run("Marshal", func(t *testing.T) {
		data, err := json.Marshal(expectedFixedAlert)
		require.NoError(t, err)
		require.JSONEq(t, expectedJSON, string(data))
	})
	t.Run("Unmarshal", func(t *testing.T) {
		var alert AlertFixed
		err := json.Unmarshal([]byte(expectedJSON), &alert)
		require.NoError(t, err)
		require.Equal(t, expectedFixedAlert, alert)
	})
}
