package file

import (
	"context"
	"encoding/json"
	"os"

	"github.com/kerpe-l/metrics-alerting-service/internal/logger"
	"github.com/kerpe-l/metrics-alerting-service/internal/model"
	"github.com/kerpe-l/metrics-alerting-service/internal/repository"
)

// Save сохраняет все метрики из хранилища в файл в формате JSON.
func Save(filename string, storage repository.Storage) error {
	gauges, counters := storage.GetAll()

	metrics := make([]model.Metrics, 0, len(gauges)+len(counters))

	for name, val := range gauges {
		v := val
		metrics = append(metrics, model.Metrics{
			ID:    name,
			MType: model.Gauge,
			Value: &v,
		})
	}

	for name, val := range counters {
		d := val
		metrics = append(metrics, model.Metrics{
			ID:    name,
			MType: model.Counter,
			Delta: &d,
		})
	}

	data, err := json.MarshalIndent(metrics, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filename, data, 0644)
}

// Load загружает метрики из файла и помещает их в хранилище.
func Load(filename string, storage repository.Storage) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		return err
	}

	var metrics []model.Metrics
	if err := json.Unmarshal(data, &metrics); err != nil {
		return err
	}

	for _, m := range metrics {
		switch m.MType {
		case model.Gauge:
			if m.Value != nil {
				if err := storage.UpdateGauge(m.ID, *m.Value); err != nil {
					return err
				}
			}
		case model.Counter:
			if m.Delta != nil {
				if err := storage.UpdateCounter(m.ID, *m.Delta); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

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

func (s *SyncStorage) UpdateGauge(name string, value float64) error {
	if err := s.Storage.UpdateGauge(name, value); err != nil {
		return err
	}
	return Save(s.filePath, s.Storage)
}

func (s *SyncStorage) UpdateCounter(name string, value int64) error {
	if err := s.Storage.UpdateCounter(name, value); err != nil {
		return err
	}
	return Save(s.filePath, s.Storage)
}

func (s *SyncStorage) UpdateBatch(ctx context.Context, metrics []model.Metrics) error {
	if err := s.Storage.UpdateBatch(ctx, metrics); err != nil {
		return err
	}
	if err := Save(s.filePath, s.Storage); err != nil {
		logger.Log.Error("Ошибка синхронного сохранения метрик: " + err.Error())
	}
	return nil
}
