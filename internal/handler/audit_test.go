package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kerpe-l/metrics-alerting-service/internal/audit"
	"github.com/kerpe-l/metrics-alerting-service/internal/model"
	"github.com/kerpe-l/metrics-alerting-service/internal/repository"
	"github.com/kerpe-l/metrics-alerting-service/internal/service"
)

// captureObserver запоминает все полученные события для проверки в тестах.
type captureObserver struct {
	mu     sync.Mutex
	events []audit.Event
}

func (o *captureObserver) Notify(_ context.Context, ev audit.Event) error {
	o.mu.Lock()
	o.events = append(o.events, ev)
	o.mu.Unlock()
	return nil
}

func (o *captureObserver) snapshot() []audit.Event {
	o.mu.Lock()
	defer o.mu.Unlock()
	out := make([]audit.Event, len(o.events))
	copy(out, o.events)
	return out
}

func newHandlerWithAudit(t *testing.T) (*MetricsHandler, *captureObserver, func()) {
	t.Helper()
	storage := repository.NewMemStorage()
	svc := service.NewMetricsService(storage)
	obs := &captureObserver{}
	pub := audit.NewPublisher(obs)
	h := &MetricsHandler{Service: svc, Audit: pub}
	cleanup := func() {
		require.NoError(t, pub.Close(context.Background()))
	}
	return h, obs, cleanup
}

func TestAudit_PublishedAfterPathUpdate(t *testing.T) {
	h, obs, cleanup := newHandlerWithAudit(t)
	defer cleanup()

	r := chi.NewRouter()
	r.Post("/update/{type}/{name}/{value}", h.UpdateHandler)
	ts := httptest.NewServer(r)
	defer ts.Close()

	resp, err := ts.Client().Post(ts.URL+"/update/gauge/Alloc/3.14", "", nil)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	require.Equal(t, http.StatusOK, resp.StatusCode)

	cleanup()
	events := obs.snapshot()
	require.Len(t, events, 1)
	assert.Equal(t, []string{"Alloc"}, events[0].Metrics)
	assert.NotEmpty(t, events[0].IPAddress)
	assert.Positive(t, events[0].Timestamp)
}

func TestAudit_NotPublishedOnError(t *testing.T) {
	h, obs, cleanup := newHandlerWithAudit(t)
	defer cleanup()

	r := chi.NewRouter()
	r.Post("/update/{type}/{name}/{value}", h.UpdateHandler)
	ts := httptest.NewServer(r)
	defer ts.Close()

	// Некорректное значение gauge — должно вернуть 400, события быть не должно.
	resp, err := ts.Client().Post(ts.URL+"/update/gauge/Alloc/not-a-number", "", nil)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)

	cleanup()
	assert.Empty(t, obs.snapshot())
}

func TestAudit_PublishedAfterJSONUpdate(t *testing.T) {
	h, obs, cleanup := newHandlerWithAudit(t)
	defer cleanup()

	r := chi.NewRouter()
	r.Post("/update/", h.UpdateJSONHandler)
	ts := httptest.NewServer(r)
	defer ts.Close()

	v := 1.23
	body, err := json.Marshal(model.Metrics{ID: "Frees", MType: model.Gauge, Value: &v})
	require.NoError(t, err)

	resp, err := ts.Client().Post(ts.URL+"/update/", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	require.Equal(t, http.StatusOK, resp.StatusCode)

	cleanup()
	events := obs.snapshot()
	require.Len(t, events, 1)
	assert.Equal(t, []string{"Frees"}, events[0].Metrics)
}

func TestAudit_PublishedAfterBatchUpdate(t *testing.T) {
	h, obs, cleanup := newHandlerWithAudit(t)
	defer cleanup()

	r := chi.NewRouter()
	r.Post("/updates/", h.UpdateBatchHandler)
	ts := httptest.NewServer(r)
	defer ts.Close()

	d1, d2 := int64(1), int64(2)
	v := 9.0
	body, err := json.Marshal([]model.Metrics{
		{ID: "C1", MType: model.Counter, Delta: &d1},
		{ID: "C2", MType: model.Counter, Delta: &d2},
		{ID: "G1", MType: model.Gauge, Value: &v},
	})
	require.NoError(t, err)

	resp, err := ts.Client().Post(ts.URL+"/updates/", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	require.Equal(t, http.StatusOK, resp.StatusCode)

	cleanup()
	events := obs.snapshot()
	require.Len(t, events, 1)
	assert.Equal(t, []string{"C1", "C2", "G1"}, events[0].Metrics)
}

func TestAudit_NoPublisherIsNoop(t *testing.T) {
	storage := repository.NewMemStorage()
	svc := service.NewMetricsService(storage)
	h := &MetricsHandler{Service: svc} // Audit == nil

	r := chi.NewRouter()
	r.Post("/update/{type}/{name}/{value}", h.UpdateHandler)
	ts := httptest.NewServer(r)
	defer ts.Close()

	resp, err := ts.Client().Post(ts.URL+"/update/gauge/X/1", "", nil)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}
