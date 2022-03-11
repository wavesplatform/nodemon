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

type alertsStorage struct {
	alertBackoff     int
	alertVacuumQuota int
	storage          map[string]alertInfo
}

func newAlertsStorage(alertBackoff, alertVacuumQuota int) *alertsStorage {
	return &alertsStorage{
		alertBackoff:     alertBackoff,
		alertVacuumQuota: alertVacuumQuota,
		storage:          make(map[string]alertInfo),
	}
}

func (s *alertsStorage) PutAlert(alert entities.Alert) bool {
	alertID := alert.ID()
	old, in := s.storage[alertID]
	if !in { // first time alert
		s.storage[alertID] = alertInfo{
			vacuumQuota:      s.alertVacuumQuota,
			repeats:          0,
			backoffThreshold: s.alertBackoff,
			alert:            alert,
		}
		return true
	}
	old.repeats += 1
	if old.repeats >= old.backoffThreshold { // backoff exceeded
		s.storage[alertID] = alertInfo{
			vacuumQuota:      s.alertVacuumQuota,
			repeats:          0,
			backoffThreshold: s.alertBackoff * old.backoffThreshold,
			alert:            alert,
		}
		return true
	}
	// we have to update quota for vacuum stage due to alert repeat
	old.vacuumQuota = s.alertVacuumQuota
	s.storage[alertID] = old
	return false
}

func (s *alertsStorage) ids() []string {
	if len(s.storage) == 0 {
		return nil
	}
	ids := make([]string, 0, len(s.storage))
	for id := range s.storage {
		ids = append(ids, id)
	}
	return ids
}

func (s *alertsStorage) Vacuum() []entities.Alert {
	var alertsFixed []entities.Alert
	for _, id := range s.ids() {
		info := s.storage[id]
		info.vacuumQuota -= 1
		if info.vacuumQuota <= 0 {
			alertsFixed = append(alertsFixed, info.alert)
			delete(s.storage, id)
		} else {
			s.storage[id] = info
		}
	}
	return alertsFixed
}
