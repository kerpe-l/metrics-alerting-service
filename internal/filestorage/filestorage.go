package filestorage

import (
	"encoding/json"
	"os"

	"github.com/kerpe-l/metrics-alerting-service/internal/model"
	"github.com/kerpe-l/metrics-alerting-service/internal/repository"
)

// Save сохраняет все метрики из хранилища в файл в формате JSON
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

// Load загружает метрики из файла и помещает их в хранилище
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
				storage.UpdateGauge(m.ID, *m.Value)
			}
		case model.Counter:
			if m.Delta != nil {
				storage.UpdateCounter(m.ID, *m.Delta)
			}
		}
	}

	return nil
}
