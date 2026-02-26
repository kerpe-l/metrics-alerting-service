package filestorage

import (
	"github.com/kerpe-l/metrics-alerting-service/internal/logger"
	"github.com/kerpe-l/metrics-alerting-service/internal/repository"
)

// SyncStorage оборачивает Storage и после каждого обновления сохраняет данные на диск.
type SyncStorage struct {
	repository.Storage
	filePath string
}

// NewSyncStorage создаёт SyncStorage, который синхронно пишет на диск после каждого обновления.
func NewSyncStorage(storage repository.Storage, filePath string) *SyncStorage {
	return &SyncStorage{
		Storage:  storage,
		filePath: filePath,
	}
}

func (s *SyncStorage) UpdateGauge(name string, value float64) {
	s.Storage.UpdateGauge(name, value)
	if err := Save(s.filePath, s.Storage); err != nil {
		logger.Log.Error("Ошибка синхронного сохранения метрик: " + err.Error())
	}
}

func (s *SyncStorage) UpdateCounter(name string, value int64) {
	s.Storage.UpdateCounter(name, value)
	if err := Save(s.filePath, s.Storage); err != nil {
		logger.Log.Error("Ошибка синхронного сохранения метрик: " + err.Error())
	}
}
