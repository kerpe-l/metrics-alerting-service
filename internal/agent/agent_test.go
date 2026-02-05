package agent

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
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
	// тестовый сервер
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	tests := []struct {
		name       string
		serverAddr string
	}{
		{
			name:       "Успешная отправка на сервер",
			serverAddr: server.URL,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewStats()
			s.Collect()

			assert.NotPanics(t, func() {
				s.Send(tt.serverAddr)
			})
		})
	}
}
