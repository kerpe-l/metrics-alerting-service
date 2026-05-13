package audit

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recordingObserver запоминает все полученные события.
type recordingObserver struct {
	mu     sync.Mutex
	events []Event
	err    error
}

func (o *recordingObserver) Notify(_ context.Context, ev Event) error {
	o.mu.Lock()
	o.events = append(o.events, ev)
	o.mu.Unlock()
	return o.err
}

func (o *recordingObserver) snapshot() []Event {
	o.mu.Lock()
	defer o.mu.Unlock()
	out := make([]Event, len(o.events))
	copy(out, o.events)
	return out
}

// blockingObserver удерживает Notify до закрытия done или отмены контекста.
type blockingObserver struct {
	started chan struct{}
	done    chan struct{}
	hits    atomic.Int32
}

func newBlockingObserver() *blockingObserver {
	return &blockingObserver{
		started: make(chan struct{}, 1),
		done:    make(chan struct{}),
	}
}

func (o *blockingObserver) Notify(ctx context.Context, _ Event) error {
	o.hits.Add(1)
	select {
	case o.started <- struct{}{}:
	default:
	}
	select {
	case <-o.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (o *blockingObserver) release() { close(o.done) }

func TestPublisher_PublishWithoutObservers(t *testing.T) {
	p := NewPublisher()
	p.Publish(Event{Timestamp: 1, Metrics: []string{"x"}, IPAddress: "1.1.1.1"})
	require.NoError(t, p.Close(context.Background()))
}

func TestPublisher_DispatchesToAllObservers(t *testing.T) {
	p := NewPublisher()
	a := &recordingObserver{}
	b := &recordingObserver{}
	p.Register(a)
	p.Register(b)

	ev := Event{Timestamp: 42, Metrics: []string{"Alloc", "Frees"}, IPAddress: "10.0.0.1"}
	p.Publish(ev)

	require.NoError(t, p.Close(context.Background()))
	assert.Equal(t, []Event{ev}, a.snapshot())
	assert.Equal(t, []Event{ev}, b.snapshot())
}

func TestPublisher_PublishIsNonBlocking(t *testing.T) {
	p := NewPublisher()
	blk := newBlockingObserver()
	p.Register(blk)

	start := time.Now()
	p.Publish(Event{Timestamp: 1})
	elapsed := time.Since(start)

	assert.Less(t, elapsed, 50*time.Millisecond, "Publish должен возвращать управление сразу")

	// Дожидаемся, что наблюдатель действительно вошёл в Notify.
	select {
	case <-blk.started:
	case <-time.After(time.Second):
		t.Fatal("наблюдатель так и не запустился")
	}

	blk.release()
	require.NoError(t, p.Close(context.Background()))
}

func TestPublisher_CloseWaitsForObservers(t *testing.T) {
	p := NewPublisher()
	rec := &recordingObserver{}

	// Наблюдатель с искусственной задержкой.
	slow := observerFunc(func(ctx context.Context, ev Event) error {
		select {
		case <-time.After(100 * time.Millisecond):
		case <-ctx.Done():
			return ctx.Err()
		}
		return rec.Notify(ctx, ev)
	})
	p.Register(slow)

	p.Publish(Event{Timestamp: 7})

	start := time.Now()
	require.NoError(t, p.Close(context.Background()))
	assert.GreaterOrEqual(t, time.Since(start), 100*time.Millisecond)
	assert.Len(t, rec.snapshot(), 1)
}

func TestPublisher_CloseRespectsContextDeadline(t *testing.T) {
	p := NewPublisher()
	blk := newBlockingObserver()
	p.Register(blk)

	p.Publish(Event{Timestamp: 1})
	<-blk.started

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := p.Close(ctx)
	assert.ErrorIs(t, err, context.DeadlineExceeded)

	// После Close внутренний контекст отменён — освобождаем наблюдателя,
	// чтобы не оставлять висящую горутину для следующих тестов.
	blk.release()
}

func TestPublisher_DropsAfterClose(t *testing.T) {
	p := NewPublisher()
	rec := &recordingObserver{}
	p.Register(rec)

	require.NoError(t, p.Close(context.Background()))
	p.Publish(Event{Timestamp: 1})

	assert.Empty(t, rec.snapshot())
}

func TestPublisher_ObserverErrorDoesNotAffectOthers(t *testing.T) {
	p := NewPublisher()
	failing := &recordingObserver{err: errors.New("boom")}
	healthy := &recordingObserver{}
	p.Register(failing)
	p.Register(healthy)

	ev := Event{Timestamp: 1, Metrics: []string{"m"}}
	p.Publish(ev)
	require.NoError(t, p.Close(context.Background()))

	assert.Equal(t, []Event{ev}, failing.snapshot())
	assert.Equal(t, []Event{ev}, healthy.snapshot())
}

// observerFunc — адаптер функции к Observer.
type observerFunc func(ctx context.Context, ev Event) error

func (f observerFunc) Notify(ctx context.Context, ev Event) error { return f(ctx, ev) }
