package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kerpe-l/metrics-alerting-service/internal/repository"
	"github.com/stretchr/testify/assert"
)

func TestMetricsHandler_UpdateHandler(t *testing.T) {
	storage := repository.NewMemStorage()
	h := &MetricsHandler{Storage: storage}
	type want struct {
		code int
	}
	tests := []struct {
		name   string
		method string
		url    string
		want   want
	}{
		{
			name:   "Успешное обновление gauge",
			method: http.MethodPost,
			url:    "/update/gauge/testGauge/100.1",
			want:   want{code: http.StatusOK},
		},
		{
			name:   "Успешное обновление counter",
			method: http.MethodPost,
			url:    "/update/counter/testCounter/10",
			want:   want{code: http.StatusOK},
		},
		{
			name:   "Некорректный метод (GET)",
			method: http.MethodGet,
			url:    "/update/gauge/testGauge/100.1",
			want:   want{code: http.StatusMethodNotAllowed},
		},
		{
			name:   "Отсутствует имя метрики",
			method: http.MethodPost,
			url:    "/update/gauge/",
			want:   want{code: http.StatusNotFound},
		},
		{
			name:   "Некорректное значение",
			method: http.MethodPost,
			url:    "/update/gauge/testGauge/none",
			want:   want{code: http.StatusBadRequest},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := httptest.NewRequest(tt.method, tt.url, nil)
			w := httptest.NewRecorder()

			h.UpdateHandler(w, request)

			assert.Equal(t, tt.want.code, w.Code, "Код ответа не совпадает для теста: %s", tt.name)
		})
	}
}
