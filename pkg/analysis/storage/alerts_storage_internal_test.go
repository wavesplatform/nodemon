package storage

import (
	"slices"
	"testing"

	"nodemon/pkg/entities"

	"github.com/neilotoole/slogt"
	"github.com/stretchr/testify/require"
)

func TestAlertsStorage(t *testing.T) {
	logger := slogt.New(t)
	var (
		alert1 = &entities.SimpleAlert{Description: "first simple alert"}
		alert2 = &entities.SimpleAlert{Description: "second simple alert"}
		alert3 = &entities.SimpleAlert{Description: "third simple alert"}
	)
	tests := []struct {
		alertBackoff        int
		alertVacuumQuota    int
		vacuumResults       [][]entities.Alert
		initialAlerts       []entities.Alert
		alerts              []entities.Alert
		alertConfirmations  alertConfirmations
		sendAlertNowResults []bool
		expectedAlertsInfo  []alertInfo
	}{
		{
			alertBackoff:        DefaultAlertBackoff,
			alertVacuumQuota:    4,
			vacuumResults:       [][]entities.Alert{{}, {}, {}},
			initialAlerts:       []entities.Alert{alert1, alert2},
			alerts:              []entities.Alert{alert3, alert1, alert1},
			alertConfirmations:  nil,
			sendAlertNowResults: []bool{true, false, true},
			expectedAlertsInfo: []alertInfo{
				{
					vacuumQuota:      4,
					repeats:          1,
					backoffThreshold: DefaultAlertBackoff * DefaultAlertBackoff,
					confirmed:        true,
					alert:            alert1,
				},
				{
					vacuumQuota:      1,
					repeats:          1,
					backoffThreshold: DefaultAlertBackoff,
					confirmed:        true,
					alert:            alert2,
				},
				{
					vacuumQuota:      4,
					repeats:          1,
					backoffThreshold: DefaultAlertBackoff,
					confirmed:        true,
					alert:            alert3,
				},
			},
		},
		{
			alertBackoff:        DefaultAlertBackoff,
			alertVacuumQuota:    2,
			vacuumResults:       [][]entities.Alert{{}, {alert1, alert2}, {}},
			initialAlerts:       []entities.Alert{alert1, alert2},
			alerts:              []entities.Alert{alert3, alert3, alert1, alert1, alert1},
			alertConfirmations:  nil,
			sendAlertNowResults: []bool{true, false, true, false, true},
			expectedAlertsInfo: []alertInfo{
				{
					vacuumQuota:      2,
					repeats:          1,
					backoffThreshold: DefaultAlertBackoff * DefaultAlertBackoff,
					confirmed:        true,
					alert:            alert1,
				},
				{
					vacuumQuota:      2,
					repeats:          2,
					backoffThreshold: DefaultAlertBackoff,
					confirmed:        true,
					alert:            alert3,
				},
			},
		},
		{
			alertBackoff:        DefaultAlertBackoff,
			alertVacuumQuota:    0,
			vacuumResults:       nil,
			initialAlerts:       nil,
			alerts:              []entities.Alert{alert1, alert1, alert1, alert2, alert3},
			alertConfirmations:  nil,
			sendAlertNowResults: []bool{true, true, true, true, true},
			expectedAlertsInfo:  nil,
		},
		{
			alertBackoff:        DefaultAlertBackoff,
			alertVacuumQuota:    1,
			vacuumResults:       nil,
			initialAlerts:       nil,
			alerts:              []entities.Alert{alert1, alert1, alert1, alert2, alert3},
			alertConfirmations:  nil,
			sendAlertNowResults: []bool{true, true, true, true, true},
			expectedAlertsInfo:  nil,
		},
		{
			alertBackoff:        DefaultAlertBackoff,
			alertVacuumQuota:    2,
			vacuumResults:       [][]entities.Alert{{}, {alert1}, {}},
			initialAlerts:       []entities.Alert{alert1, alert2, alert1},
			alerts:              []entities.Alert{alert3, alert3, alert2, alert1, alert1, alert1, alert1, alert1},
			sendAlertNowResults: []bool{false, true, false, false, true, false, true, false},
			alertConfirmations: newAlertConfirmations(AlertConfirmationsValue{
				AlertType:     entities.SimpleAlertType,
				Confirmations: 2,
			}),
			expectedAlertsInfo: []alertInfo{
				{
					vacuumQuota:      2,
					repeats:          2,
					backoffThreshold: DefaultAlertBackoff * DefaultAlertBackoff,
					confirmed:        true,
					alert:            alert1,
				},
				{
					vacuumQuota:      2,
					repeats:          1,
					backoffThreshold: 0,
					confirmed:        false,
					alert:            alert2,
				},
				{
					vacuumQuota:      2,
					repeats:          1,
					backoffThreshold: DefaultAlertBackoff,
					confirmed:        true,
					alert:            alert3,
				},
			},
		},
	}
	for i, test := range tests {
		tcNum := i + 1
		require.Equal(t, len(test.alerts), len(test.sendAlertNowResults),
			"failed constraint in test case#%d", tcNum,
		)

		storage := newAlertsStorage(test.alertBackoff, test.alertVacuumQuota, test.alertConfirmations, logger)
		for _, alert := range test.initialAlerts {
			storage.PutAlert(alert)
		}
		for j, expectedVacuumed := range test.vacuumResults {
			actualVacuumed := storage.Vacuum()
			require.ElementsMatch(t, expectedVacuumed, actualVacuumed, "test case#%d vacuum#%d", tcNum, j+1)
		}
		for j, alert := range test.alerts {
			sendAlertNow := storage.PutAlert(alert)
			require.Equal(t, test.sendAlertNowResults[j], sendAlertNow, "test case#%d alert#%d", tcNum, j+1)
		}
		actualInfos := slices.Collect(storage.internalStorage.infos())
		require.ElementsMatch(t, test.expectedAlertsInfo, actualInfos, "test case#%d", tcNum)
	}
}
