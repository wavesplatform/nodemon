package analysis

import (
	"nodemon/pkg/entities"
)

const (
	defaultAlertVacuumQuota = 5
	defaultAlertBackoff     = 2
)

type alertInfo struct {
	vacuumQuota      int
	repeats          int
	backoffThreshold int
	confirmed        bool
	alert            entities.Alert
}

type alertsInternalStorage map[string]alertInfo

func (s alertsInternalStorage) ids() []string {
	if len(s) == 0 {
		return nil
	}
	ids := make([]string, 0, len(s))
	for id := range s {
		ids = append(ids, id)
	}
	return ids
}

func (s alertsInternalStorage) infos() []alertInfo {
	if len(s) == 0 {
		return nil
	}
	infos := make([]alertInfo, 0, len(s))
	for _, info := range s {
		infos = append(infos, info)
	}
	return infos
}

type alertsStorage struct {
	alertBackoff          int
	alertVacuumQuota      int
	requiredConfirmations alertConfirmations
	internalStorage       alertsInternalStorage
}

type alertConfirmations map[entities.AlertType]int

func newAlertsStorage(alertBackoff, alertVacuumQuota int, requiredConfirmations alertConfirmations) *alertsStorage {
	return &alertsStorage{
		alertBackoff:          alertBackoff,
		alertVacuumQuota:      alertVacuumQuota,
		requiredConfirmations: requiredConfirmations,
		internalStorage:       make(alertsInternalStorage),
	}
}

func (s *alertsStorage) PutAlert(alert entities.Alert) (needSendAlert bool) {
	if s.alertVacuumQuota <= 1 { // no need to save alerts which can't outlive even one vacuum stage
		return true
	}
	var (
		alertID = alert.ID()
		old     = s.internalStorage[alertID]
		repeats = old.repeats + 1
	)
	if !old.confirmed && repeats >= s.requiredConfirmations[alert.Type()] { // send confirmed alert
		s.internalStorage[alertID] = alertInfo{
			vacuumQuota:      s.alertVacuumQuota,
			repeats:          1, // now it's a confirmed alert, so reset repeats counter
			backoffThreshold: s.alertBackoff,
			confirmed:        true,
			alert:            alert,
		}
		return true
	}
	if old.confirmed && repeats > old.backoffThreshold { // backoff exceeded, reset repeats and increase backoff
		s.internalStorage[alertID] = alertInfo{
			vacuumQuota:      s.alertVacuumQuota,
			repeats:          1,
			backoffThreshold: s.alertBackoff * old.backoffThreshold,
			confirmed:        true,
			alert:            alert,
		}
		return true
	}
	s.internalStorage[alertID] = alertInfo{
		vacuumQuota:      s.alertVacuumQuota,
		repeats:          repeats,
		backoffThreshold: old.backoffThreshold,
		confirmed:        old.confirmed,
		alert:            alert,
	}
	return false
}

func (s *alertsStorage) Vacuum() []entities.Alert {
	var alertsFixed []entities.Alert
	for _, id := range s.internalStorage.ids() {
		info := s.internalStorage[id]
		info.vacuumQuota -= 1
		if info.vacuumQuota <= 0 {
			if info.confirmed {
				alertsFixed = append(alertsFixed, info.alert)
			}
			delete(s.internalStorage, id)
		} else {
			s.internalStorage[id] = info
		}
	}
	return alertsFixed
}
