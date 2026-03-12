package file

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kerpe-l/metrics-alerting-service/internal/model"
	"github.com/kerpe-l/metrics-alerting-service/internal/repository"
)

func TestSave(t *testing.T) {
	storage := repository.NewMemStorage()
	require.NoError(t, storage.UpdateGauge("Temperature", 36.6))
	require.NoError(t, storage.UpdateGauge("Pressure", 760.0))
	require.NoError(t, storage.UpdateCounter("Requests", 100))
	require.NoError(t, storage.UpdateCounter("Errors", 5))

	path := filepath.Join(t.TempDir(), "metrics.json")

	err := Save(path, storage)
	require.NoError(t, err)

	data, err := os.ReadFile(path)
	require.NoError(t, err)

	var metrics []model.Metrics
	err = json.Unmarshal(data, &metrics)
	require.NoError(t, err)

	assert.Len(t, metrics, 4)

	byID := make(map[string]model.Metrics)
	for _, m := range metrics {
		byID[m.ID] = m
	}

	// gauge
	assert.Equal(t, model.Gauge, byID["Temperature"].MType)
	assert.InDelta(t, 36.6, *byID["Temperature"].Value, 0.001)

	assert.Equal(t, model.Gauge, byID["Pressure"].MType)
	assert.InDelta(t, 760.0, *byID["Pressure"].Value, 0.001)

	// counter
	assert.Equal(t, model.Counter, byID["Requests"].MType)
	assert.Equal(t, int64(100), *byID["Requests"].Delta)

	assert.Equal(t, model.Counter, byID["Errors"].MType)
	assert.Equal(t, int64(5), *byID["Errors"].Delta)
}

func TestLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "metrics.json")

	metrics := []model.Metrics{
		{ID: "CPU", MType: model.Gauge, Value: ptrFloat64(55.5)},
		{ID: "Hits", MType: model.Counter, Delta: ptrInt64(42)},
	}
	data, err := json.Marshal(metrics)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, data, 0644))

	storage := repository.NewMemStorage()
	err = Load(path, storage)
	require.NoError(t, err)

	val, ok := storage.GetGauge("CPU")
	assert.True(t, ok)
	assert.InDelta(t, 55.5, val, 0.001)

	delta, ok := storage.GetCounter("Hits")
	assert.True(t, ok)
	assert.Equal(t, int64(42), delta)
}

func TestSaveAndLoad_RoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "metrics.json")

	// сохраняем
	original := repository.NewMemStorage()
	require.NoError(t, original.UpdateGauge("Alloc", 123456.78))
	require.NoError(t, original.UpdateCounter("PollCount", 99))

	require.NoError(t, Save(path, original))

	// загружаем в новое хранилище
	restored := repository.NewMemStorage()
	require.NoError(t, Load(path, restored))

	// сравниваем
	val, ok := restored.GetGauge("Alloc")
	assert.True(t, ok)
	assert.InDelta(t, 123456.78, val, 0.001)

	delta, ok := restored.GetCounter("PollCount")
	assert.True(t, ok)
	assert.Equal(t, int64(99), delta)
}

func TestLoad_FileNotFound(t *testing.T) {
	storage := repository.NewMemStorage()
	err := Load("/nonexistent/path/file.json", storage)
	assert.Error(t, err)
}

func TestLoad_InvalidJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.json")
	require.NoError(t, os.WriteFile(path, []byte("not json"), 0644))

	storage := repository.NewMemStorage()
	err := Load(path, storage)
	assert.Error(t, err)
}

func TestSave_EmptyStorage(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty.json")
	storage := repository.NewMemStorage()

	require.NoError(t, Save(path, storage))

	data, err := os.ReadFile(path)
	require.NoError(t, err)

	var metrics []model.Metrics
	require.NoError(t, json.Unmarshal(data, &metrics))
	assert.Empty(t, metrics)
}

func ptrFloat64(v float64) *float64 { return &v }
func ptrInt64(v int64) *int64       { return &v }
