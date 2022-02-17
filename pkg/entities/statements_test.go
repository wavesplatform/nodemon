package entities

import (
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNodeStatementStateHashName(t *testing.T) {
	// check for JSON index in buntdb
	nodeStatementType := reflect.TypeOf(NodeStatement{})
	tests := []struct {
		structFieldName      string
		expectedJSONTagValue string
	}{
		{"StateHash", NodeStatementStateHashJSONFieldName},
	}
	for _, test := range tests {
		field, ok := nodeStatementType.FieldByName(test.structFieldName)
		require.True(t, ok)
		split := strings.Split(field.Tag.Get("json"), ",")
		require.Equal(t, split[0], test.expectedJSONTagValue)
	}
}
