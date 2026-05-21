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

// closingObserver — наблюдатель, реализующий io.Closer.
type closingObserver struct {
	recordingObserver
	closed   atomic.Bool
	closeErr error
}

func (o *closingObserver) Close() error {
	o.closed.Store(true)
	return o.closeErr
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

// withBufferSize временно подменяет ёмкость очереди наблюдателя.
func withBufferSize(t *testing.T, n int) {
	t.Helper()
	orig := bufferSize
	bufferSize = n
	t.Cleanup(func() { bufferSize = orig })
}

func TestPublisher_PublishWithoutObservers(t *testing.T) {
	p := NewPublisher()
	p.Publish(Event{Timestamp: 1, Metrics: []string{"x"}, IPAddress: "1.1.1.1"})
	require.NoError(t, p.Close(context.Background()))
}

func TestPublisher_DispatchesToAllObservers(t *testing.T) {
	a := &recordingObserver{}
	b := &recordingObserver{}
	p := NewPublisher(a, b)

	ev := Event{Timestamp: 42, Metrics: []string{"Alloc", "Frees"}, IPAddress: "10.0.0.1"}
	p.Publish(ev)

	require.NoError(t, p.Close(context.Background()))
	assert.Equal(t, []Event{ev}, a.snapshot())
	assert.Equal(t, []Event{ev}, b.snapshot())
}

func TestPublisher_PublishIsNonBlocking(t *testing.T) {
	blk := newBlockingObserver()
	p := NewPublisher(blk)

	start := time.Now()
	p.Publish(Event{Timestamp: 1})
	assert.Less(t, time.Since(start), 50*time.Millisecond, "Publish должен возвращать управление сразу")

	select {
	case <-blk.started:
	case <-time.After(time.Second):
		t.Fatal("наблюдатель так и не запустился")
	}

	blk.release()
	require.NoError(t, p.Close(context.Background()))
}

func TestPublisher_CloseDeliversBufferedEvents(t *testing.T) {
	rec := &recordingObserver{}
	slow := observerFunc(func(ctx context.Context, ev Event) error {
		select {
		case <-time.After(50 * time.Millisecond):
		case <-ctx.Done():
			return ctx.Err()
		}
		return rec.Notify(ctx, ev)
	})
	p := NewPublisher(slow)

	p.Publish(Event{Timestamp: 1})
	p.Publish(Event{Timestamp: 2})

	require.NoError(t, p.Close(context.Background()))
	assert.Len(t, rec.snapshot(), 2)
}

func TestPublisher_CloseRespectsContextDeadline(t *testing.T) {
	blk := newBlockingObserver()
	p := NewPublisher(blk)

	p.Publish(Event{Timestamp: 1})
	<-blk.started

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := p.Close(ctx)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestPublisher_DropsAfterClose(t *testing.T) {
	rec := &recordingObserver{}
	p := NewPublisher(rec)

	require.NoError(t, p.Close(context.Background()))
	p.Publish(Event{Timestamp: 1})

	assert.Empty(t, rec.snapshot())
}

func TestPublisher_ObserverErrorDoesNotAffectOthers(t *testing.T) {
	failing := &recordingObserver{err: errors.New("boom")}
	healthy := &recordingObserver{}
	p := NewPublisher(failing, healthy)

	ev := Event{Timestamp: 1, Metrics: []string{"m"}}
	p.Publish(ev)
	require.NoError(t, p.Close(context.Background()))

	assert.Equal(t, []Event{ev}, failing.snapshot())
	assert.Equal(t, []Event{ev}, healthy.snapshot())
}

func TestPublisher_DropsEventsWhenBufferFull(t *testing.T) {
	withBufferSize(t, 1)
	blk := newBlockingObserver()
	p := NewPublisher(blk)

	// Воркер забирает первое событие и блокируется в Notify.
	p.Publish(Event{Timestamp: 1})
	<-blk.started

	p.Publish(Event{Timestamp: 2}) // занимает буфер (ёмкость 1)
	p.Publish(Event{Timestamp: 3}) // буфер полон — отбрасывается
	p.Publish(Event{Timestamp: 4}) // отбрасывается

	blk.release()
	require.NoError(t, p.Close(context.Background()))

	// Обработаны только событие из воркера и одно буферизированное.
	assert.Equal(t, int32(2), blk.hits.Load())
}

func TestPublisher_CloseClosesObservers(t *testing.T) {
	o := &closingObserver{}
	p := NewPublisher(o)

	require.NoError(t, p.Close(context.Background()))
	assert.True(t, o.closed.Load())
}

func TestPublisher_CloseAggregatesObserverCloseErrors(t *testing.T) {
	err1 := errors.New("close-1")
	err2 := errors.New("close-2")
	o1 := &closingObserver{closeErr: err1}
	o2 := &closingObserver{closeErr: err2}
	p := NewPublisher(o1, o2)

	err := p.Close(context.Background())
	assert.ErrorIs(t, err, err1)
	assert.ErrorIs(t, err, err2)
}

// observerFunc — адаптер функции к Observer.
type observerFunc func(ctx context.Context, ev Event) error

func (f observerFunc) Notify(ctx context.Context, ev Event) error { return f(ctx, ev) }
