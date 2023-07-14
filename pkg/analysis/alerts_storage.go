package analysis

import (
	"nodemon/pkg/entities"

	"github.com/wavesplatform/gowaves/pkg/crypto"
	"go.uber.org/zap"
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

type alertsInternalStorage map[crypto.Digest]alertInfo

func (s alertsInternalStorage) ids() []crypto.Digest {
	if len(s) == 0 {
		return nil
	}
	ids := make([]crypto.Digest, 0, len(s))
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
	logger                *zap.Logger
}

type alertConfirmations map[entities.AlertType]int

const (
	heightAlertConfirmationsDefault = 2
)

type alertConfirmationsValue struct {
	alertType     entities.AlertType
	confirmations int
}

func newAlertConfirmations(customConfirmations ...alertConfirmationsValue) alertConfirmations {
	confirmations := alertConfirmations{
		entities.SimpleAlertType:        0,
		entities.UnreachableAlertType:   0,
		entities.IncompleteAlertType:    0,
		entities.InvalidHeightAlertType: 0,
		entities.HeightAlertType:        0,
		entities.StateHashAlertType:     0,
		entities.AlertFixedType:         0,
		entities.BaseTargetAlertType:    0,
		entities.InternalErrorAlertType: 0,
	}
	for _, cc := range customConfirmations {
		confirmations[cc.alertType] = cc.confirmations
	}
	return confirmations
}

func newAlertsStorage(
	alertBackoff, alertVacuumQuota int,
	requiredConfirmations alertConfirmations,
	logger *zap.Logger,
) *alertsStorage {
	return &alertsStorage{
		alertBackoff:          alertBackoff,
		alertVacuumQuota:      alertVacuumQuota,
		requiredConfirmations: requiredConfirmations,
		internalStorage:       make(alertsInternalStorage),
		logger:                logger,
	}
}

func (s *alertsStorage) PutAlert(alert entities.Alert) bool {
	if s.alertVacuumQuota <= 1 { // no need to save alerts which can't outlive even one vacuum stage
		return true
	}
	var (
		alertID = alert.ID()
		old     = s.internalStorage[alertID]
		repeats = old.repeats + 1
	)
	defer func() {
		info := s.internalStorage[alertID]
		s.logger.Info("An alert was put into storage",
			zap.Stringer("alert", info.alert),
			zap.Int("backoffThreshold", info.backoffThreshold),
			zap.Int("repeats", info.repeats),
			zap.Bool("confirmed", info.confirmed),
		)
	}()

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
		info.vacuumQuota--
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
