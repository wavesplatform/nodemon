package analysis

import (
	"testing"

	"github.com/stretchr/testify/require"
	"nodemon/pkg/entities"
)

func TestAlertsStorage(t *testing.T) {
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
		sendAlertNowResults []bool
		expectedAlertsInfo  []alertInfo
	}{
		{
			alertBackoff:        defaultAlertBackoff,
			alertVacuumQuota:    4,
			vacuumResults:       [][]entities.Alert{{}, {}, {}},
			initialAlerts:       []entities.Alert{alert1, alert2},
			alerts:              []entities.Alert{alert3, alert1, alert1},
			sendAlertNowResults: []bool{true, false, true},
			expectedAlertsInfo: []alertInfo{
				{
					vacuumQuota:      4,
					repeats:          0,
					backoffThreshold: defaultAlertBackoff * defaultAlertBackoff,
					alert:            alert1,
				},
				{
					vacuumQuota:      1,
					repeats:          0,
					backoffThreshold: defaultAlertBackoff,
					alert:            alert2,
				},
				{
					vacuumQuota:      4,
					repeats:          0,
					backoffThreshold: defaultAlertBackoff,
					alert:            alert3,
				},
			},
		},
		{
			alertBackoff:        defaultAlertBackoff,
			alertVacuumQuota:    2,
			vacuumResults:       [][]entities.Alert{{}, {alert1, alert2}, {}},
			initialAlerts:       []entities.Alert{alert1, alert2},
			alerts:              []entities.Alert{alert3, alert3, alert1, alert1, alert1},
			sendAlertNowResults: []bool{true, false, true, false, true},
			expectedAlertsInfo: []alertInfo{
				{
					vacuumQuota:      2,
					repeats:          0,
					backoffThreshold: defaultAlertBackoff * defaultAlertBackoff,
					alert:            alert1,
				},
				{
					vacuumQuota:      2,
					repeats:          1,
					backoffThreshold: defaultAlertBackoff,
					alert:            alert3,
				},
			},
		},
		{
			alertBackoff:        defaultAlertBackoff,
			alertVacuumQuota:    0,
			vacuumResults:       nil,
			initialAlerts:       nil,
			alerts:              []entities.Alert{alert1, alert1, alert1, alert2, alert3},
			sendAlertNowResults: []bool{true, true, true, true, true},
			expectedAlertsInfo:  nil,
		},
		{
			alertBackoff:        defaultAlertBackoff,
			alertVacuumQuota:    1,
			vacuumResults:       nil,
			initialAlerts:       nil,
			alerts:              []entities.Alert{alert1, alert1, alert1, alert2, alert3},
			sendAlertNowResults: []bool{true, true, true, true, true},
			expectedAlertsInfo:  nil,
		},
	}
	for i, test := range tests {
		tcNum := i + 1
		require.Equal(t, len(test.alerts), len(test.sendAlertNowResults),
			"failed constraint in test case#%d", tcNum,
		)

		storage := newAlertsStorage(test.alertBackoff, test.alertVacuumQuota)
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
		actualInfos := storage.internalStorage.infos()
		require.ElementsMatch(t, test.expectedAlertsInfo, actualInfos, "test case#%d", tcNum)
	}
}
