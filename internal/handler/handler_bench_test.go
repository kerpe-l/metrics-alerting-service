package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/kerpe-l/metrics-alerting-service/internal/model"
	"github.com/kerpe-l/metrics-alerting-service/internal/repository"
	"github.com/kerpe-l/metrics-alerting-service/internal/service"
)

func benchHandler() *MetricsHandler {
	storage := repository.NewMemStorage()
	ctx := context.Background()
	for i := 0; i < 200; i++ {
		_ = storage.UpdateGauge(ctx, fmt.Sprintf("g%d", i), float64(i)*1.5)
		_ = storage.UpdateCounter(ctx, fmt.Sprintf("c%d", i), int64(i))
	}
	svc := service.NewMetricsService(storage)
	return &MetricsHandler{Service: svc}
}

func benchBatchPayload(n int) []byte {
	metrics := make([]model.Metrics, 0, n)
	for i := 0; i < n; i++ {
		v := float64(i) * 1.5
		metrics = append(metrics, model.Metrics{
			ID:    fmt.Sprintf("g%d", i),
			MType: model.Gauge,
			Value: &v,
		})
	}
	data, _ := json.Marshal(metrics)
	return data
}

func BenchmarkUpdateBatchHandler(b *testing.B) {
	h := benchHandler()
	r := chi.NewRouter()
	r.Post("/updates/", h.UpdateBatchHandler)

	payload := benchBatchPayload(50)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodPost, "/updates/", bytes.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}

func BenchmarkUpdateJSONHandler(b *testing.B) {
	h := benchHandler()
	r := chi.NewRouter()
	r.Post("/update/", h.UpdateJSONHandler)

	v := 3.14
	payload, _ := json.Marshal(model.Metrics{ID: "x", MType: model.Gauge, Value: &v})
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodPost, "/update/", bytes.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}

func BenchmarkValueJSONHandler(b *testing.B) {
	h := benchHandler()
	r := chi.NewRouter()
	r.Post("/value/", h.ValueJSONHandler)

	payload, _ := json.Marshal(model.Metrics{ID: "g10", MType: model.Gauge})
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodPost, "/value/", bytes.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}

func BenchmarkRootHandler(b *testing.B) {
	h := benchHandler()
	r := chi.NewRouter()
	r.Get("/", h.RootHandler)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}
