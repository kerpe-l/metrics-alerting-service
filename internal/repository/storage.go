// Package repository определяет интерфейс хранилища метрик Storage и его
// in-memory реализацию MemStorage.
package repository

import (
	"context"
	"errors"
	"maps"
	"sync"

	"github.com/kerpe-l/metrics-alerting-service/internal/model"
)

// ErrNoDB означает, что хранилище не поддерживает проверку соединения с БД.
var ErrNoDB = errors.New("no database connection")

type Storage interface {
	UpdateGauge(ctx context.Context, name string, value float64) error
	UpdateCounter(ctx context.Context, name string, value int64) error
	UpdateBatch(ctx context.Context, metrics []model.Metrics) error
	GetGauge(ctx context.Context, name string) (float64, bool)
	GetCounter(ctx context.Context, name string) (int64, bool)
	GetAll(ctx context.Context) (map[string]float64, map[string]int64)
	Ping(ctx context.Context) error
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

func (ms *MemStorage) UpdateGauge(_ context.Context, name string, value float64) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	ms.gauges[name] = value
	return nil
}

func (ms *MemStorage) UpdateCounter(_ context.Context, name string, value int64) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	ms.counters[name] += value
	return nil
}

func (ms *MemStorage) UpdateBatch(_ context.Context, metrics []model.Metrics) error {
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

func (ms *MemStorage) GetGauge(_ context.Context, name string) (float64, bool) {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	v, ok := ms.gauges[name]
	return v, ok
}

func (ms *MemStorage) GetCounter(_ context.Context, name string) (int64, bool) {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	v, ok := ms.counters[name]
	return v, ok
}

func (ms *MemStorage) Ping(_ context.Context) error {
	return ErrNoDB
}

func (ms *MemStorage) GetAll(_ context.Context) (map[string]float64, map[string]int64) {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	// возвращаем копии, чтобы избежать гонки данных
	// оригинальные мапы нельзя отдавать наружу, так как там доступ к ним не защищен мьютексом
	return maps.Clone(ms.gauges), maps.Clone(ms.counters)
}
