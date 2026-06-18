// Package grpcserver реализует gRPC-сервис Metrics: приём метрик батчем
// (UpdateMetrics) и делегирование их в бизнес-логику. Маппинг доменных ошибок
// в коды gRPC собран в одном месте (mapError) — по аналогии с HTTP-хендлером.
package grpcserver

import (
	"context"
	"errors"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/kerpe-l/metrics-alerting-service/internal/logger"
	"github.com/kerpe-l/metrics-alerting-service/internal/model"
	pb "github.com/kerpe-l/metrics-alerting-service/internal/proto"
	"github.com/kerpe-l/metrics-alerting-service/internal/service"
)

// errUnknownType — тип метрики в запросе не gauge и не counter (битый enum
// на проводе). Маппится в codes.InvalidArgument.
var errUnknownType = errors.New("unknown metric type")

// metricsUpdater — узкая зависимость сервера: обновление батча метрик.
// Реализуется service.MetricsService.
type metricsUpdater interface {
	UpdateBatch(ctx context.Context, metrics []model.Metrics) error
}

// Server реализует gRPC-сервис Metrics поверх metricsUpdater.
type Server struct {
	pb.UnimplementedMetricsServer
	svc metricsUpdater
}

// New создаёт gRPC-сервер метрик, делегирующий обновления в svc.
func New(svc metricsUpdater) *Server {
	return &Server{svc: svc}
}

// UpdateMetrics принимает батч метрик и сохраняет его через UpdateBatch.
// Конвертирует proto-представление в доменную модель; при битом типе метрики
// или ошибке сервиса возвращает соответствующий код gRPC.
func (s *Server) UpdateMetrics(ctx context.Context, req *pb.UpdateMetricsRequest) (*pb.UpdateMetricsResponse, error) {
	metrics := make([]model.Metrics, 0, len(req.GetMetrics()))
	for _, m := range req.GetMetrics() {
		converted, err := toModel(m)
		if err != nil {
			return nil, mapError(err)
		}
		metrics = append(metrics, converted)
	}

	if err := s.svc.UpdateBatch(ctx, metrics); err != nil {
		return nil, mapError(err)
	}

	return &pb.UpdateMetricsResponse{}, nil
}

// toModel конвертирует proto-метрику в доменную. Тип берётся из enum, значение —
// из delta (counter) или value (gauge). Неизвестный enum даёт errUnknownType.
func toModel(m *pb.Metric) (model.Metrics, error) {
	metric := model.Metrics{ID: m.GetId()}
	switch m.GetType() {
	case pb.Metric_GAUGE:
		v := m.GetValue()
		metric.MType = model.Gauge
		metric.Value = &v
	case pb.Metric_COUNTER:
		d := m.GetDelta()
		metric.MType = model.Counter
		metric.Delta = &d
	default:
		return model.Metrics{}, fmt.Errorf("%w: %d", errUnknownType, m.GetType())
	}
	return metric, nil
}

// mapError маппит доменные ошибки на коды gRPC. Единая точка маппинга:
// ошибки валидации → InvalidArgument, прочее → Internal (с логом).
func mapError(err error) error {
	switch {
	case errors.Is(err, errUnknownType),
		errors.Is(err, service.ErrEmptyBatch),
		errors.Is(err, service.ErrInvalidType),
		errors.Is(err, service.ErrEmptyID),
		errors.Is(err, service.ErrMissingValue),
		errors.Is(err, service.ErrMissingDelta):
		return status.Error(codes.InvalidArgument, err.Error())
	default:
		logger.Log.Error("grpc UpdateMetrics: " + err.Error())
		return status.Error(codes.Internal, err.Error())
	}
}
