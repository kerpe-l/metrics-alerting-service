package agent

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kerpe-l/metrics-alerting-service/internal/model"
)

func TestNewCollector(t *testing.T) {
	tests := []struct {
		name string
	}{
		{
			name: "Инициализация пустого коллектора",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewCollector()
			assert.NotNil(t, got)
			assert.NotNil(t, got.runtimeMetrics)
			assert.Equal(t, int64(0), got.pollCount)
		})
	}
}

func TestCollector_Collect(t *testing.T) {
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
			c := NewCollector()
			for i := 0; i < tt.iterations; i++ {
				c.Collect()
			}

			assert.Equal(t, tt.wantPollCount, c.pollCount)
			assert.NotEmpty(t, c.runtimeMetrics)
			assert.Contains(t, c.runtimeMetrics, "Alloc")
		})
	}
}

func TestCollector_Metrics(t *testing.T) {
	c := NewCollector()
	c.Collect()

	metrics := c.Metrics()

	expectedCount := len(c.runtimeMetrics) + len(c.extraMetrics) + 2 // gauges + extra + RandomValue + PollCount
	assert.Equal(t, expectedCount, len(metrics))

	var foundPollCount bool
	for _, m := range metrics {
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

func TestCollector_CollectExtra(t *testing.T) {
	tests := []struct {
		name           string
		wantTotalMem   bool
		wantFreeMem    bool
		wantCPU        bool
	}{
		{
			name:         "Сбор дополнительных метрик (gopsutil)",
			wantTotalMem: true,
			wantFreeMem:  true,
			wantCPU:      true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewCollector()
			c.CollectExtra()

			assert.NotEmpty(t, c.extraMetrics)
			if tt.wantTotalMem {
				assert.Contains(t, c.extraMetrics, "TotalMemory")
				assert.Greater(t, c.extraMetrics["TotalMemory"], float64(0))
			}
			if tt.wantFreeMem {
				assert.Contains(t, c.extraMetrics, "FreeMemory")
			}
			if tt.wantCPU {
				assert.Contains(t, c.extraMetrics, "CPUutilization1")
			}
		})
	}
}

func TestCollector_MetricsWithExtra(t *testing.T) {
	c := NewCollector()
	c.Collect()
	c.CollectExtra()

	metrics := c.Metrics()

	expectedCount := len(c.runtimeMetrics) + len(c.extraMetrics) + 2
	assert.Equal(t, expectedCount, len(metrics))

	byID := make(map[string]model.Metrics)
	for _, m := range metrics {
		byID[m.ID] = m
	}

	assert.Contains(t, byID, "TotalMemory")
	assert.Equal(t, model.Gauge, byID["TotalMemory"].MType)
	assert.NotNil(t, byID["TotalMemory"].Value)

	assert.Contains(t, byID, "FreeMemory")
	assert.Contains(t, byID, "CPUutilization1")
	assert.Contains(t, byID, "Alloc")
	assert.Contains(t, byID, "PollCount")
}

func TestCollector_ConcurrentAccess(t *testing.T) {
	c := NewCollector()

	done := make(chan struct{})

	go func() {
		for i := 0; i < 100; i++ {
			c.Collect()
		}
		done <- struct{}{}
	}()

	go func() {
		for i := 0; i < 100; i++ {
			c.CollectExtra()
		}
		done <- struct{}{}
	}()

	go func() {
		for i := 0; i < 100; i++ {
			c.Metrics()
		}
		done <- struct{}{}
	}()

	<-done
	<-done
	<-done
}

func TestSender_Send(t *testing.T) {
	var received []model.Metrics

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/updates/", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "gzip", r.Header.Get("Content-Encoding"))

		gz, err := gzip.NewReader(r.Body)
		if err != nil {
			t.Fatalf("failed to create gzip reader: %v", err)
		}
		defer func() { _ = gz.Close() }()

		if err := json.NewDecoder(gz).Decode(&received); err != nil {
			t.Fatalf("failed to decode batch: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	c := NewCollector()
	c.Collect()

	s := NewSender(server.URL, "")
	s.Send(context.Background(), c.Metrics())

	expectedCount := len(c.runtimeMetrics) + len(c.extraMetrics) + 2 // gauges + extra + RandomValue + PollCount
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
