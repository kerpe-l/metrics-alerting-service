package grpcserver

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/kerpe-l/metrics-alerting-service/internal/model"
	pb "github.com/kerpe-l/metrics-alerting-service/internal/proto"
	"github.com/kerpe-l/metrics-alerting-service/internal/service"
)

// fakeUpdater — ручной фейк metricsUpdater: запоминает полученный батч и
// отдаёт заданную ошибку.
type fakeUpdater struct {
	received []model.Metrics
	err      error
	called   bool
}

func (f *fakeUpdater) UpdateBatch(_ context.Context, metrics []model.Metrics) error {
	f.called = true
	f.received = metrics
	return f.err
}

func ptrFloat(v float64) *float64 { return &v }
func ptrInt(v int64) *int64       { return &v }

func TestServer_UpdateMetrics_Conversion(t *testing.T) {
	tests := []struct {
		name string
		in   *pb.Metric
		want model.Metrics
	}{
		{
			name: "gauge",
			in:   (&pb.Metric_builder{Id: "Alloc", Type: pb.Metric_GAUGE, Value: 42.5}).Build(),
			want: model.Metrics{ID: "Alloc", MType: model.Gauge, Value: ptrFloat(42.5)},
		},
		{
			name: "counter",
			in:   (&pb.Metric_builder{Id: "PollCount", Type: pb.Metric_COUNTER, Delta: 7}).Build(),
			want: model.Metrics{ID: "PollCount", MType: model.Counter, Delta: ptrInt(7)},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fake := &fakeUpdater{}
			srv := New(fake)

			_, err := srv.UpdateMetrics(context.Background(),
				(&pb.UpdateMetricsRequest_builder{Metrics: []*pb.Metric{tc.in}}).Build())
			require.NoError(t, err)
			require.Len(t, fake.received, 1)
			assert.Equal(t, tc.want, fake.received[0])
		})
	}
}

func TestServer_UpdateMetrics_Errors(t *testing.T) {
	tests := []struct {
		name       string
		req        *pb.UpdateMetricsRequest
		svcErr     error
		wantCode   codes.Code
		wantCalled bool
	}{
		{
			name:       "unknown type rejected before delegating",
			req:        (&pb.UpdateMetricsRequest_builder{Metrics: []*pb.Metric{(&pb.Metric_builder{Id: "x", Type: pb.Metric_MType(99)}).Build()}}).Build(),
			wantCode:   codes.InvalidArgument,
			wantCalled: false,
		},
		{
			name:       "empty batch maps to InvalidArgument",
			req:        (&pb.UpdateMetricsRequest_builder{}).Build(),
			svcErr:     service.ErrEmptyBatch,
			wantCode:   codes.InvalidArgument,
			wantCalled: true,
		},
		{
			name:       "service failure maps to Internal",
			req:        (&pb.UpdateMetricsRequest_builder{Metrics: []*pb.Metric{(&pb.Metric_builder{Id: "x", Type: pb.Metric_GAUGE}).Build()}}).Build(),
			svcErr:     errors.New("db down"),
			wantCode:   codes.Internal,
			wantCalled: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fake := &fakeUpdater{err: tc.svcErr}
			srv := New(fake)

			_, err := srv.UpdateMetrics(context.Background(), tc.req)
			require.Error(t, err)
			assert.Equal(t, tc.wantCode, status.Code(err))
			assert.Equal(t, tc.wantCalled, fake.called)
		})
	}
}
