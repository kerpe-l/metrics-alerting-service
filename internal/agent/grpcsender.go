package agent

import (
	"context"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/kerpe-l/metrics-alerting-service/internal/logger"
	"github.com/kerpe-l/metrics-alerting-service/internal/model"
	pb "github.com/kerpe-l/metrics-alerting-service/internal/proto"
	"github.com/kerpe-l/metrics-alerting-service/internal/retry"
)

// metadataKeyRealIP — ключ метаданных с IP-адресом агента (gRPC хранит ключи
// в нижнем регистре).
const metadataKeyRealIP = "x-real-ip"

// GRPCSender отправляет метрики на сервер по gRPC. Реализует BatchSender,
// поэтому подставляется в Pool вместо HTTP-Sender.
type GRPCSender struct {
	client pb.MetricsClient
	realIP string
}

// NewGRPCSender создаёт gRPC-отправитель поверх готового соединения cc.
// realIP проставляется в метаданные x-real-ip каждого запроса (пусто = не ставится);
// жизненным циклом соединения управляет вызывающий.
func NewGRPCSender(cc grpc.ClientConnInterface, realIP string) *GRPCSender {
	return &GRPCSender{
		client: pb.NewMetricsClient(cc),
		realIP: realIP,
	}
}

// Send отправляет батч метрик одним вызовом UpdateMetrics.
func (s *GRPCSender) Send(ctx context.Context, metrics []model.Metrics) {
	if len(metrics) == 0 {
		return
	}

	req := (&pb.UpdateMetricsRequest_builder{Metrics: toProto(metrics)}).Build()

	if s.realIP != "" {
		ctx = metadata.AppendToOutgoingContext(ctx, metadataKeyRealIP, s.realIP)
	}

	err := retry.Do(ctx, func() error {
		_, sendErr := s.client.UpdateMetrics(ctx, req)
		return sendErr
	}, isRetriableGRPCError)
	if err != nil {
		logger.Log.Error("failed to send metrics over grpc", zap.Error(err))
	}
}

// toProto конвертирует доменные метрики в proto-представление. Значение берётся
// по типу: value для gauge, delta для counter. Сборка через Opaque-билдер —
// прямого доступа к полям proto-структур нет.
func toProto(metrics []model.Metrics) []*pb.Metric {
	out := make([]*pb.Metric, 0, len(metrics))
	for _, m := range metrics {
		b := pb.Metric_builder{Id: m.ID}
		switch m.MType {
		case model.Gauge:
			b.Type = pb.Metric_GAUGE
			if m.Value != nil {
				b.Value = *m.Value
			}
		case model.Counter:
			b.Type = pb.Metric_COUNTER
			if m.Delta != nil {
				b.Delta = *m.Delta
			}
		}
		out = append(out, b.Build())
	}
	return out
}

// isRetriableGRPCError повторяет запрос только при codes.Unavailable
// (сервер недоступен/перезапускается).
func isRetriableGRPCError(err error) bool {
	return status.Code(err) == codes.Unavailable
}
