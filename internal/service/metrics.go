// Package service реализует бизнес-логику работы с метриками: валидацию,
// маршрутизацию по типу (gauge/counter) и делегирование в хранилище.
package service

import (
	"context"
	"errors"
	"strconv"

	"github.com/kerpe-l/metrics-alerting-service/internal/model"
	"github.com/kerpe-l/metrics-alerting-service/internal/repository"
)

// Sentinel-ошибки сервиса. Хендлеры сравнивают их через errors.Is и маппят
// в HTTP-статусы в одном месте (writeServiceError).
var (
	// ErrMetricNotFound — запрошенная метрика отсутствует в хранилище.
	ErrMetricNotFound = errors.New("metric not found")
	// ErrInvalidType — неизвестный тип метрики (не counter и не gauge).
	ErrInvalidType = errors.New("unknown metric type")
	// ErrMissingValue — для gauge не передано значение.
	ErrMissingValue = errors.New("value is required for gauge")
	// ErrMissingDelta — для counter не передана дельта.
	ErrMissingDelta = errors.New("delta is required for counter")
	// ErrEmptyID — не указано имя метрики.
	ErrEmptyID = errors.New("metric ID is required")
	// ErrEmptyBatch — передан пустой батч метрик.
	ErrEmptyBatch = errors.New("empty batch")
)

// MetricsService описывает бизнес-логику работы с метриками.
type MetricsService interface {
	// Update обновляет метрику по типу: для gauge используется value,
	// для counter — delta. Возвращает ErrInvalidType при неизвестном типе.
	Update(ctx context.Context, name, mType string, value float64, delta int64) error
	// UpdateJSON обновляет метрику из model.Metrics и возвращает её
	// актуальное состояние.
	UpdateJSON(ctx context.Context, metric model.Metrics) (model.Metrics, error)
	// GetValue возвращает текущее значение метрики в виде строки.
	GetValue(ctx context.Context, name, mType string) (string, error)
	// GetJSON возвращает метрику с заполненным актуальным значением.
	GetJSON(ctx context.Context, metric model.Metrics) (model.Metrics, error)
	// UpdateBatch обновляет несколько метрик за один вызов.
	UpdateBatch(ctx context.Context, metrics []model.Metrics) error
	// GetAll возвращает копии всех gauge- и counter-метрик.
	GetAll(ctx context.Context) (map[string]float64, map[string]int64)
	// Ping проверяет доступность хранилища (БД).
	Ping(ctx context.Context) error
}

type metricsService struct {
	repo repository.Storage
}

// NewMetricsService создаёт новый сервис метрик
func NewMetricsService(repo repository.Storage) MetricsService {
	return &metricsService{repo: repo}
}

func (s *metricsService) Update(ctx context.Context, name, mType string, value float64, delta int64) error {
	switch mType {
	case model.Gauge:
		return s.repo.UpdateGauge(ctx, name, value)
	case model.Counter:
		return s.repo.UpdateCounter(ctx, name, delta)
	default:
		return ErrInvalidType
	}
}

func (s *metricsService) UpdateJSON(ctx context.Context, metric model.Metrics) (model.Metrics, error) {
	if metric.ID == "" {
		return metric, ErrEmptyID
	}

	switch metric.MType {
	case model.Gauge:
		if metric.Value == nil {
			return metric, ErrMissingValue
		}
		if err := s.repo.UpdateGauge(ctx, metric.ID, *metric.Value); err != nil {
			return metric, err
		}
		val, ok := s.repo.GetGauge(ctx, metric.ID)
		if !ok {
			return metric, ErrMetricNotFound
		}
		metric.Value = &val

	case model.Counter:
		if metric.Delta == nil {
			return metric, ErrMissingDelta
		}
		if err := s.repo.UpdateCounter(ctx, metric.ID, *metric.Delta); err != nil {
			return metric, err
		}
		val, ok := s.repo.GetCounter(ctx, metric.ID)
		if !ok {
			return metric, ErrMetricNotFound
		}
		metric.Delta = &val

	default:
		return metric, ErrInvalidType
	}
	return metric, nil
}

func (s *metricsService) GetValue(ctx context.Context, name, mType string) (string, error) {
	switch mType {
	case model.Gauge:
		val, ok := s.repo.GetGauge(ctx, name)
		if !ok {
			return "", ErrMetricNotFound
		}
		return strconv.FormatFloat(val, 'f', -1, 64), nil

	case model.Counter:
		val, ok := s.repo.GetCounter(ctx, name)
		if !ok {
			return "", ErrMetricNotFound
		}
		return strconv.FormatInt(val, 10), nil

	default:
		return "", ErrInvalidType
	}
}

func (s *metricsService) GetJSON(ctx context.Context, metric model.Metrics) (model.Metrics, error) {
	if metric.ID == "" {
		return metric, ErrEmptyID
	}

	switch metric.MType {
	case model.Gauge:
		val, ok := s.repo.GetGauge(ctx, metric.ID)
		if !ok {
			return metric, ErrMetricNotFound
		}
		metric.Value = &val

	case model.Counter:
		val, ok := s.repo.GetCounter(ctx, metric.ID)
		if !ok {
			return metric, ErrMetricNotFound
		}
		metric.Delta = &val

	default:
		return metric, ErrInvalidType
	}
	return metric, nil
}

func (s *metricsService) UpdateBatch(ctx context.Context, metrics []model.Metrics) error {
	if len(metrics) == 0 {
		return ErrEmptyBatch
	}
	return s.repo.UpdateBatch(ctx, metrics)
}

func (s *metricsService) GetAll(ctx context.Context) (map[string]float64, map[string]int64) {
	return s.repo.GetAll(ctx)
}

func (s *metricsService) Ping(ctx context.Context) error {
	return s.repo.Ping(ctx)
}
