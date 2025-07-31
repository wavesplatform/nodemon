package entities_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"nodemon/pkg/entities"
)

func TestFixedAlertJSON(t *testing.T) {
	t.Run("ShadowedTypeDoesNotImplementUnmarshaler", func(t *testing.T) {
		type shadowed entities.AlertFixed
		var v shadowed // see trick in AlertFixed.UnmarshalJSON
		_, typeImplements := any(v).(json.Unmarshaler)
		require.False(t, typeImplements, "type must not implement Unmarshaler")
		_, pointerImplements := any(&v).(json.Unmarshaler)
		require.False(t, pointerImplements, "pointer must not implement Unmarshaler")
	})
	t.Run("ShadowedTypeDoesNotImplementMarshaler", func(t *testing.T) {
		type shadowed entities.AlertFixed
		var v shadowed // see trick in AlertFixed.UnmarshalJSON
		_, typeImplements := any(v).(json.Marshaler)
		require.False(t, typeImplements, "type must not implement Marshaler")
		_, pointerImplements := any(&v).(json.Marshaler)
		require.False(t, pointerImplements, "pointer must not implement Marshaler")
	})
	var (
		expectedFixedAlert = entities.AlertFixed{
			Timestamp: 1,
			Fixed: &entities.UnreachableAlert{
				Timestamp: 2,
				Node:      "node1",
			},
		}
		expectedJSON = `{"timestamp":1,"fixed":{"timestamp":2,"node":"node1"},"fixed_alert_type":2}`
	)
	t.Run("Marshal", func(t *testing.T) {
		// AlertFixed should implement json.Marshaler due to difference between type assertion inside json.Marshal function:
		// - type implements ==> pointer implements
		// - pointer implements != type implements
		// Hence json.Marshal won't use AlertFixed.MarshalJSON implementation if you pass AlertFixed by type
		// in case when AlertFixed implements json.Marshaler by pointer.
		_ = json.Marshaler(entities.AlertFixed{})

		data, err := json.Marshal(expectedFixedAlert)
		require.NoError(t, err)
		require.JSONEq(t, expectedJSON, string(data))
	})
	t.Run("Unmarshal", func(t *testing.T) {
		// AlertFixed implements json.Unmarshaler by pointer
		_ = json.Unmarshaler(&entities.AlertFixed{})

		var alert entities.AlertFixed
		err := json.Unmarshal([]byte(expectedJSON), &alert)
		require.NoError(t, err)
		require.Equal(t, expectedFixedAlert, alert)
	})
}
