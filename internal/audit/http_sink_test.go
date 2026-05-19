package audit

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHTTPSink_PostsJSONEvent(t *testing.T) {
	var (
		gotMethod      string
		gotContentType string
		gotBody        []byte
	)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotContentType = r.Header.Get("Content-Type")
		b, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		gotBody = b
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	sink := NewHTTPSink(ts.URL)
	ev := Event{Timestamp: 100, Metrics: []string{"Alloc", "Frees"}, IPAddress: "1.2.3.4"}
	require.NoError(t, sink.Notify(t.Context(), ev))

	assert.Equal(t, http.MethodPost, gotMethod)
	assert.Equal(t, "application/json", gotContentType)

	var decoded Event
	require.NoError(t, json.Unmarshal(gotBody, &decoded))
	assert.Equal(t, ev, decoded)
}

func TestHTTPSink_NonRetriableStatusReturnsErrorWithoutRetry(t *testing.T) {
	var hits atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer ts.Close()

	sink := NewHTTPSink(ts.URL)
	err := sink.Notify(t.Context(), Event{Timestamp: 1})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "400")
	assert.Equal(t, int32(1), hits.Load(), "4xx не должен повторяться")
}

func TestHTTPSink_RetriesOn5xxThenSucceeds(t *testing.T) {
	var hits atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if hits.Add(1) == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	sink := NewHTTPSink(ts.URL)
	require.NoError(t, sink.Notify(t.Context(), Event{Timestamp: 1}))
	assert.Equal(t, int32(2), hits.Load(), "первый 5xx должен повлечь повтор")
}

func TestHTTPSink_RetriesExhaustedReturnsError(t *testing.T) {
	var hits atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()

	sink := NewHTTPSink(ts.URL)
	err := sink.Notify(ctx, Event{Timestamp: 1})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "500")
	assert.GreaterOrEqual(t, hits.Load(), int32(1))
}

func TestHTTPSink_ContextCancelAbortsRequest(t *testing.T) {
	// Сервер сигналит о получении запроса и держит соединение, пока тест не
	// освободит хендлер: важно, чтобы Notify вернулся именно по отмене
	// клиентского контекста. release закрывается до ts.Close (порядок defer
	// LIFO), иначе ts.Close завис бы в ожидании зависшего хендлера.
	received := make(chan struct{})
	release := make(chan struct{})
	ts := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		close(received)
		<-release
	}))
	defer ts.Close()
	defer close(release)

	sink := NewHTTPSink(ts.URL)
	ctx, cancel := context.WithCancel(t.Context())

	var wg sync.WaitGroup
	errCh := make(chan error, 1)
	wg.Add(1)
	go func() {
		defer wg.Done()
		errCh <- sink.Notify(ctx, Event{Timestamp: 1})
	}()

	<-received // запрос гарантированно дошёл до сервера
	cancel()
	wg.Wait() // горутина точно завершилась

	err := <-errCh
	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestHTTPSink_InvalidURLReturnsError(t *testing.T) {
	sink := NewHTTPSink("http://127.0.0.1:1") // порт, на котором никто не слушает
	sink.client.Timeout = 200 * time.Millisecond
	err := sink.Notify(t.Context(), Event{Timestamp: 1})
	assert.Error(t, err)
}

func TestHTTPSink_RequestUsedExactlyOnce(t *testing.T) {
	var hits atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusAccepted)
	}))
	defer ts.Close()

	sink := NewHTTPSink(ts.URL)
	require.NoError(t, sink.Notify(t.Context(), Event{Timestamp: 1}))
	assert.Equal(t, int32(1), hits.Load())
}
