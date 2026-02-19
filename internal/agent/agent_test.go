package agent

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kerpe-l/metrics-alerting-service/internal/model"
)

func TestNewStats(t *testing.T) {
	tests := []struct {
		name string
	}{
		{
			name: "Инициализация пустого хранилища",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewStats()
			assert.NotNil(t, got)
			assert.NotNil(t, got.RuntimeMetrics)
			assert.Equal(t, int64(0), got.PollCount)
		})
	}
}

func TestStats_Collect(t *testing.T) {
	tests := []struct {
		name          string
		iterations    int
		wantPollCount int64
	}{
		{
			name:          "Первый сбор метрик",
			iterations:    1,
			wantPollCount: 1,
		},
		{
			name:          "Множественный сбор (проверка инкремента)",
			iterations:    3,
			wantPollCount: 3,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewStats()
			for i := 0; i < tt.iterations; i++ {
				s.Collect()
			}

			assert.Equal(t, tt.wantPollCount, s.PollCount)
			assert.NotEmpty(t, s.RuntimeMetrics)
			assert.Contains(t, s.RuntimeMetrics, "Alloc")
		})
	}
}

func TestStats_Send(t *testing.T) {
	var received []model.Metrics

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/update/", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var m model.Metrics
		if err := json.NewDecoder(r.Body).Decode(&m); err == nil {
			received = append(received, m)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	s := NewStats()
	s.Collect()
	s.Send(server.URL)

	expectedCount := len(s.RuntimeMetrics) + 2 // gauges + RandomValue + PollCount
	assert.Equal(t, expectedCount, len(received))

	var foundPollCount bool
	for _, m := range received {
		assert.NotEmpty(t, m.ID)
		if m.ID == "PollCount" {
			assert.Equal(t, model.Counter, m.MType)
			assert.NotNil(t, m.Delta)
			foundPollCount = true
		} else {
			assert.Equal(t, model.Gauge, m.MType)
			assert.NotNil(t, m.Value)
		}
	}
	assert.True(t, foundPollCount)
}
