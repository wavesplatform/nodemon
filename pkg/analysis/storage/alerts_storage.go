package storage

import (
	"iter"
	"maps"

	"nodemon/pkg/entities"

	"github.com/wavesplatform/gowaves/pkg/crypto"
	"go.uber.org/zap"
)

const (
	DefaultAlertVacuumQuota = 5
	DefaultAlertBackoff     = 2
)

type alertInfo struct {
	vacuumQuota      int
	repeats          int
	backoffThreshold int
	confirmed        bool
	alert            entities.Alert
}

type alertsInternalStorage map[crypto.Digest]alertInfo

func (s alertsInternalStorage) ids() iter.Seq[crypto.Digest] {
	return maps.Keys(s)
}

func (s alertsInternalStorage) infos() iter.Seq[alertInfo] {
	return maps.Values(s)
}

type AlertsStorage struct {
	alertBackoff          int
	alertVacuumQuota      int
	requiredConfirmations alertConfirmations
	internalStorage       alertsInternalStorage
	logger                *zap.Logger
}

type alertConfirmations map[entities.AlertType]int

type AlertConfirmationsValue struct {
	AlertType     entities.AlertType
	Confirmations int
}

func newAlertConfirmations(customConfirmations ...AlertConfirmationsValue) alertConfirmations {
	confirmations := make(alertConfirmations, len(customConfirmations))
	for _, cc := range customConfirmations {
		confirmations[cc.AlertType] = cc.Confirmations
	}
	return confirmations
}

type AlertsStorageOption func(*AlertsStorage)

func AlertBackoff(backoff int) AlertsStorageOption {
	return func(s *AlertsStorage) { s.alertBackoff = backoff }
}

func AlertVacuumQuota(quota int) AlertsStorageOption {
	return func(s *AlertsStorage) { s.alertVacuumQuota = quota }
}

func AlertConfirmations(confirmations ...AlertConfirmationsValue) AlertsStorageOption {
	return func(s *AlertsStorage) { s.requiredConfirmations = newAlertConfirmations(confirmations...) }
}

func NewAlertsStorage(logger *zap.Logger, opts ...AlertsStorageOption) *AlertsStorage {
	s := newAlertsStorage(DefaultAlertBackoff, DefaultAlertVacuumQuota, newAlertConfirmations(), logger)
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func newAlertsStorage(
	alertBackoff, alertVacuumQuota int,
	requiredConfirmations alertConfirmations,
	logger *zap.Logger,
) *AlertsStorage {
	return &AlertsStorage{
		alertBackoff:          alertBackoff,
		alertVacuumQuota:      alertVacuumQuota,
		requiredConfirmations: requiredConfirmations,
		internalStorage:       make(alertsInternalStorage),
		logger:                logger,
	}
}

func (s *AlertsStorage) PutAlert(alert entities.Alert) bool {
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

func (s *AlertsStorage) Vacuum() []entities.Alert {
	var alertsFixed []entities.Alert
	for id := range s.internalStorage.ids() {
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
