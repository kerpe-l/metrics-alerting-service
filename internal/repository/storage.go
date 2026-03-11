package repository

import (
	"maps"
	"sync"

	"github.com/kerpe-l/metrics-alerting-service/internal/model"
)

type Storage interface {
	UpdateGauge(name string, value float64)
	UpdateCounter(name string, value int64)
	UpdateBatch(metrics []model.Metrics) error
	GetGauge(name string) (float64, bool)
	GetCounter(name string) (int64, bool)
	GetAll() (map[string]float64, map[string]int64)
}

type MemStorage struct {
	gauges   map[string]float64
	counters map[string]int64
	mu       sync.RWMutex
}

func NewMemStorage() *MemStorage {
	return &MemStorage{
		gauges:   make(map[string]float64),
		counters: make(map[string]int64),
	}
}

func (ms *MemStorage) UpdateGauge(name string, value float64) {
	ms.mu.Lock() // блокировка для записи
	defer ms.mu.Unlock()
	ms.gauges[name] = value
}

func (ms *MemStorage) UpdateCounter(name string, value int64) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	ms.counters[name] += value
}

func (ms *MemStorage) UpdateBatch(metrics []model.Metrics) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	for _, m := range metrics {
		switch m.MType {
		case model.Gauge:
			if m.Value != nil {
				ms.gauges[m.ID] = *m.Value
			}
		case model.Counter:
			if m.Delta != nil {
				ms.counters[m.ID] += *m.Delta
			}
		}
	}
	return nil
}

func (ms *MemStorage) GetGauge(name string) (float64, bool) {
	ms.mu.RLock() // блокировка для чтения
	defer ms.mu.RUnlock()
	v, ok := ms.gauges[name]
	return v, ok
}

func (ms *MemStorage) GetCounter(name string) (int64, bool) {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	v, ok := ms.counters[name]
	return v, ok
}

func (ms *MemStorage) GetAll() (map[string]float64, map[string]int64) {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	// возвращаем копии, чтобы избежать гонки данных
	// оригинальные мапы нельзя отдавать наружу, так как там доступ к ним не защищен мьютексом
	return maps.Clone(ms.gauges), maps.Clone(ms.counters)
}
