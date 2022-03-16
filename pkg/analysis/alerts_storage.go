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
	alertBackoff     int
	alertVacuumQuota int
	internalStorage  alertsInternalStorage
}

func newAlertsStorage(alertBackoff, alertVacuumQuota int) *alertsStorage {
	return &alertsStorage{
		alertBackoff:     alertBackoff,
		alertVacuumQuota: alertVacuumQuota,
		internalStorage:  make(alertsInternalStorage),
	}
}

func (s *alertsStorage) PutAlert(alert entities.Alert) bool {
	if s.alertVacuumQuota <= 1 { // no need to save alerts which can't outlive even one vacuum stage
		return true
	}
	alertID := alert.ID()
	old, in := s.internalStorage[alertID]
	if !in { // first time alert
		s.internalStorage[alertID] = alertInfo{
			vacuumQuota:      s.alertVacuumQuota,
			repeats:          0,
			backoffThreshold: s.alertBackoff,
			alert:            alert,
		}
		return true
	}
	old.repeats += 1
	if old.repeats >= old.backoffThreshold { // backoff exceeded
		s.internalStorage[alertID] = alertInfo{
			vacuumQuota:      s.alertVacuumQuota,
			repeats:          0,
			backoffThreshold: s.alertBackoff * old.backoffThreshold,
			alert:            alert,
		}
		return true
	}
	// we have to update quota for vacuum stage due to alert repeat
	old.vacuumQuota = s.alertVacuumQuota
	s.internalStorage[alertID] = old
	return false
}

func (s *alertsStorage) Vacuum() []entities.Alert {
	var alertsFixed []entities.Alert
	for _, id := range s.internalStorage.ids() {
		info := s.internalStorage[id]
		info.vacuumQuota -= 1
		if info.vacuumQuota <= 0 {
			alertsFixed = append(alertsFixed, info.alert)
			delete(s.internalStorage, id)
		} else {
			s.internalStorage[id] = info
		}
	}
	return alertsFixed
}
