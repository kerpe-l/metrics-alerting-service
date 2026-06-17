package agent

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/kerpe-l/metrics-alerting-service/internal/model"
)

// mockSender записывает полученные батчи. Если release != nil, каждый Send
// блокируется до закрытия release либо до отмены ctx (тогда батч считается
// недоставленным — как оборванный HTTP-запрос).
type mockSender struct {
	mu      sync.Mutex
	got     [][]model.Metrics
	release chan struct{}
}

func (s *mockSender) Send(ctx context.Context, metrics []model.Metrics) {
	if s.release != nil {
		select {
		case <-s.release:
		case <-ctx.Done():
			return
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.got = append(s.got, metrics)
}

func (s *mockSender) delivered() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.got)
}

// testBatch возвращает батч из одной gauge-метрики.
func testBatch(name string) []model.Metrics {
	v := 1.0
	return []model.Metrics{{ID: name, MType: model.Gauge, Value: &v}}
}

func TestPoolShutdownDeliversAllBatches(t *testing.T) {
	tests := []struct {
		name      string
		workers   int
		queueSize int
		batches   int
	}{
		{name: "один воркер", workers: 1, queueSize: 1, batches: 5},
		{name: "несколько воркеров", workers: 3, queueSize: 3, batches: 10},
		{name: "очередь больше числа батчей", workers: 2, queueSize: 16, batches: 4},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			sender := &mockSender{}
			pool := NewPool(sender, tc.workers, tc.queueSize)

			for i := 0; i < tc.batches; i++ {
				if !pool.Submit(testBatch("m")) {
					t.Fatalf("Submit(батч %d) = false, ожидали true", i)
				}
			}

			if err := pool.Shutdown(context.Background()); err != nil {
				t.Fatalf("Shutdown() = %v, ожидали nil", err)
			}
			if got := sender.delivered(); got != tc.batches {
				t.Errorf("доставлено %d батчей, ожидали %d", got, tc.batches)
			}
		})
	}
}

func TestPoolShutdownDrainsQueuedBatches(t *testing.T) {
	// Воркер один и заблокирован в Send, ещё два батча лежат в очереди.
	// После разблокировки Shutdown обязан дождаться доставки всех трёх.
	sender := &mockSender{release: make(chan struct{})}
	pool := NewPool(sender, 1, 2)

	for i := 0; i < 3; i++ {
		if !pool.Submit(testBatch("m")) {
			t.Fatalf("Submit(батч %d) = false, ожидали true", i)
		}
	}

	close(sender.release)
	if err := pool.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown() = %v, ожидали nil", err)
	}
	if got := sender.delivered(); got != 3 {
		t.Errorf("доставлено %d батчей, ожидали 3", got)
	}
}

func TestPoolShutdownTimeoutAbortsHangingSend(t *testing.T) {
	// release никогда не закрывается — Send зависает до отмены контекста.
	sender := &mockSender{release: make(chan struct{})}
	pool := NewPool(sender, 1, 1)

	if !pool.Submit(testBatch("m")) {
		t.Fatal("Submit() = false, ожидали true")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := pool.Shutdown(ctx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Shutdown() = %v, ожидали context.DeadlineExceeded", err)
	}
	if got := sender.delivered(); got != 0 {
		t.Errorf("доставлено %d батчей, ожидали 0 (отправка оборвана)", got)
	}
}

func TestPoolSubmitAfterShutdown(t *testing.T) {
	sender := &mockSender{}
	pool := NewPool(sender, 1, 1)

	if err := pool.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown() = %v, ожидали nil", err)
	}
	if pool.Submit(testBatch("m")) {
		t.Error("Submit() после Shutdown = true, ожидали false")
	}
}

func TestPoolShutdownTwice(t *testing.T) {
	sender := &mockSender{}
	pool := NewPool(sender, 2, 2)

	if err := pool.Shutdown(context.Background()); err != nil {
		t.Fatalf("первый Shutdown() = %v, ожидали nil", err)
	}
	if err := pool.Shutdown(context.Background()); err != nil {
		t.Fatalf("повторный Shutdown() = %v, ожидали nil", err)
	}
}
