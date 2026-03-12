package service

import (
	"context"
	"errors"
	"strconv"

	"github.com/kerpe-l/metrics-alerting-service/internal/model"
	"github.com/kerpe-l/metrics-alerting-service/internal/repository"
)

var (
	ErrMetricNotFound = errors.New("metric not found")
	ErrInvalidType    = errors.New("unknown metric type")
	ErrMissingValue   = errors.New("value is required for gauge")
	ErrMissingDelta   = errors.New("delta is required for counter")
	ErrEmptyID        = errors.New("metric ID is required")
	ErrEmptyBatch     = errors.New("empty batch")
)

// MetricsService описывает бизнес-логику работы с метриками
type MetricsService interface {
	Update(name, mType string, value float64, delta int64) error
	UpdateJSON(metric model.Metrics) (model.Metrics, error)
	GetValue(name, mType string) (string, error)
	GetJSON(metric model.Metrics) (model.Metrics, error)
	UpdateBatch(ctx context.Context, metrics []model.Metrics) error
	GetAll() (map[string]float64, map[string]int64)
}

type metricsService struct {
	repo repository.Storage
}

// NewMetricsService создаёт новый сервис метрик
func NewMetricsService(repo repository.Storage) MetricsService {
	return &metricsService{repo: repo}
}

func (s *metricsService) Update(name, mType string, value float64, delta int64) error {
	switch mType {
	case model.Gauge:
		s.repo.UpdateGauge(name, value)
	case model.Counter:
		s.repo.UpdateCounter(name, delta)
	default:
		return ErrInvalidType
	}
	return nil
}

func (s *metricsService) UpdateJSON(metric model.Metrics) (model.Metrics, error) {
	if metric.ID == "" {
		return metric, ErrEmptyID
	}

	switch metric.MType {
	case model.Gauge:
		if metric.Value == nil {
			return metric, ErrMissingValue
		}
		s.repo.UpdateGauge(metric.ID, *metric.Value)
		val, ok := s.repo.GetGauge(metric.ID)
		if !ok {
			return metric, ErrMetricNotFound
		}
		metric.Value = &val

	case model.Counter:
		if metric.Delta == nil {
			return metric, ErrMissingDelta
		}
		s.repo.UpdateCounter(metric.ID, *metric.Delta)
		val, ok := s.repo.GetCounter(metric.ID)
		if !ok {
			return metric, ErrMetricNotFound
		}
		metric.Delta = &val

	default:
		return metric, ErrInvalidType
	}
	return metric, nil
}

func (s *metricsService) GetValue(name, mType string) (string, error) {
	switch mType {
	case model.Gauge:
		val, ok := s.repo.GetGauge(name)
		if !ok {
			return "", ErrMetricNotFound
		}
		return strconv.FormatFloat(val, 'f', -1, 64), nil

	case model.Counter:
		val, ok := s.repo.GetCounter(name)
		if !ok {
			return "", ErrMetricNotFound
		}
		return strconv.FormatInt(val, 10), nil

	default:
		return "", ErrInvalidType
	}
}

func (s *metricsService) GetJSON(metric model.Metrics) (model.Metrics, error) {
	if metric.ID == "" {
		return metric, ErrEmptyID
	}

	switch metric.MType {
	case model.Gauge:
		val, ok := s.repo.GetGauge(metric.ID)
		if !ok {
			return metric, ErrMetricNotFound
		}
		metric.Value = &val

	case model.Counter:
		val, ok := s.repo.GetCounter(metric.ID)
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

func (s *metricsService) GetAll() (map[string]float64, map[string]int64) {
	return s.repo.GetAll()
}
