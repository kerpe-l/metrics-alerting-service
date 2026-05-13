package audit

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
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
	require.NoError(t, sink.Notify(context.Background(), ev))

	assert.Equal(t, http.MethodPost, gotMethod)
	assert.Equal(t, "application/json", gotContentType)

	var decoded Event
	require.NoError(t, json.Unmarshal(gotBody, &decoded))
	assert.Equal(t, ev, decoded)
}

func TestHTTPSink_NonSuccessStatusReturnsError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	sink := NewHTTPSink(ts.URL)
	err := sink.Notify(context.Background(), Event{Timestamp: 1})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

func TestHTTPSink_ContextCancelAbortsRequest(t *testing.T) {
	// Сервер удерживает ответ дольше, чем длится тест: важно, чтобы Notify
	// вернулся именно по отмене клиентского контекста.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(5 * time.Second):
			w.WriteHeader(http.StatusOK)
		case <-r.Context().Done():
		}
	}))
	defer ts.Close()

	sink := NewHTTPSink(ts.URL)
	ctx, cancel := context.WithCancel(context.Background())

	errCh := make(chan error, 1)
	go func() { errCh <- sink.Notify(ctx, Event{Timestamp: 1}) }()

	time.Sleep(20 * time.Millisecond)
	cancel()

	select {
	case err := <-errCh:
		require.Error(t, err)
		assert.ErrorIs(t, err, context.Canceled)
	case <-time.After(2 * time.Second):
		t.Fatal("Notify не вернулся после отмены контекста")
	}
}

func TestHTTPSink_InvalidURLReturnsError(t *testing.T) {
	sink := NewHTTPSink("http://127.0.0.1:1") // порт, на котором никто не слушает
	sink.client.Timeout = 200 * time.Millisecond
	err := sink.Notify(context.Background(), Event{Timestamp: 1})
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
	require.NoError(t, sink.Notify(context.Background(), Event{Timestamp: 1}))
	assert.Equal(t, int32(1), hits.Load())
}
