package analysis

import (
	"testing"

	"github.com/stretchr/testify/require"
	"nodemon/pkg/entities"
)

func TestAlertsStorage_PutAlert(t *testing.T) {
	var (
		alert1 = &entities.SimpleAlert{Description: "first simple alert"}
		alert2 = &entities.SimpleAlert{Description: "second simple alert"}
		alert3 = &entities.SimpleAlert{Description: "third simple alert"}
	)
	tests := []struct {
		alertBackoff        int
		alertVacuumQuota    int
		vacuumRuns          int
		initialAlerts       []entities.Alert
		alerts              []entities.Alert
		sendAlertNowResults []bool
		expectedAlertsInfo  []alertInfo
	}{
		{
			alertBackoff:        defaultAlertBackoff,
			alertVacuumQuota:    4,
			vacuumRuns:          3,
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
	}
	for i, test := range tests {
		tcNum := i + 1

		storage := newAlertsStorage(test.alertBackoff, test.alertVacuumQuota)
		for _, alert := range test.initialAlerts {
			storage.PutAlert(alert)
		}
		for j := 0; j < test.vacuumRuns; j++ {
			storage.Vacuum()
		}
		for j, alert := range test.alerts {
			sendAlertNow := storage.PutAlert(alert)
			require.Equal(t, test.sendAlertNowResults[j], sendAlertNow, "test case#%d alert#%d", tcNum, j+1)
		}
		actualInfos := storage.internalStorage.infos()
		require.ElementsMatch(t, test.expectedAlertsInfo, actualInfos)
	}

}
