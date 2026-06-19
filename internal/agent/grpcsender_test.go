package agent

import (
	"context"
	"net"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/test/bufconn"

	"github.com/kerpe-l/metrics-alerting-service/internal/model"
	pb "github.com/kerpe-l/metrics-alerting-service/internal/proto"
)

// fakeMetricsServer запоминает последний запрос и его метаданные.
type fakeMetricsServer struct {
	pb.UnimplementedMetricsServer
	mu       sync.Mutex
	received *pb.UpdateMetricsRequest
	md       metadata.MD
	calls    int
}

func (f *fakeMetricsServer) UpdateMetrics(ctx context.Context, req *pb.UpdateMetricsRequest) (*pb.UpdateMetricsResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.received = req
	f.md, _ = metadata.FromIncomingContext(ctx)
	f.calls++
	return &pb.UpdateMetricsResponse{}, nil
}

// startFakeServer поднимает gRPC-сервер на bufconn и возвращает фейк и
// подключённого к нему клиента. Очистка регистрируется через t.Cleanup.
func startFakeServer(t *testing.T) (*fakeMetricsServer, *grpc.ClientConn) {
	t.Helper()

	lis := bufconn.Listen(1024 * 1024)
	srv := grpc.NewServer()
	fake := &fakeMetricsServer{}
	pb.RegisterMetricsServer(srv, fake)

	go func() {
		_ = srv.Serve(lis)
	}()

	conn, err := grpc.NewClient(
		"passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)

	t.Cleanup(func() {
		_ = conn.Close()
		srv.Stop()
	})

	return fake, conn
}

func TestGRPCSender_Conversion(t *testing.T) {
	value := 42.5
	delta := int64(7)

	tests := []struct {
		name     string
		in       model.Metrics
		wantType pb.Metric_MType
		wantVal  float64
		wantDel  int64
	}{
		{
			name:     "gauge",
			in:       model.Metrics{ID: "Alloc", MType: model.Gauge, Value: &value},
			wantType: pb.Metric_GAUGE,
			wantVal:  42.5,
		},
		{
			name:     "counter",
			in:       model.Metrics{ID: "PollCount", MType: model.Counter, Delta: &delta},
			wantType: pb.Metric_COUNTER,
			wantDel:  7,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fake, conn := startFakeServer(t)
			sender := NewGRPCSender(conn, "")

			sender.Send(context.Background(), []model.Metrics{tc.in})

			fake.mu.Lock()
			defer fake.mu.Unlock()
			require.NotNil(t, fake.received)
			require.Len(t, fake.received.GetMetrics(), 1)

			got := fake.received.GetMetrics()[0]
			assert.Equal(t, tc.in.ID, got.GetId())
			assert.Equal(t, tc.wantType, got.GetType())
			assert.InDelta(t, tc.wantVal, got.GetValue(), 1e-9)
			assert.Equal(t, tc.wantDel, got.GetDelta())
		})
	}
}

func TestGRPCSender_Metadata(t *testing.T) {
	tests := []struct {
		name   string
		realIP string
		want   []string // ожидаемые значения x-real-ip (nil = ключа нет)
	}{
		{name: "realIP проставляется в метаданные", realIP: "10.1.2.3", want: []string{"10.1.2.3"}},
		{name: "пустой realIP — ключа нет", realIP: "", want: nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fake, conn := startFakeServer(t)
			sender := NewGRPCSender(conn, tc.realIP)

			sender.Send(context.Background(), []model.Metrics{
				{ID: "Alloc", MType: model.Gauge, Value: new(float64)},
			})

			fake.mu.Lock()
			defer fake.mu.Unlock()
			assert.Equal(t, tc.want, fake.md.Get(metadataKeyRealIP))
		})
	}
}

func TestGRPCSender_EmptyBatch(t *testing.T) {
	fake, conn := startFakeServer(t)
	sender := NewGRPCSender(conn, "10.1.2.3")

	sender.Send(context.Background(), nil)

	fake.mu.Lock()
	defer fake.mu.Unlock()
	assert.Zero(t, fake.calls)
}
