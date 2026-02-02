package handler

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/kerpe-l/metrics-alerting-service/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testRequest(t *testing.T, ts *httptest.Server, method, path string) (*http.Response, string) {
	req, err := http.NewRequest(method, ts.URL+path, nil)
	require.NoError(t, err)

	resp, err := ts.Client().Do(req)
	require.NoError(t, err)

	respBody, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	return resp, string(respBody)
}

func TestMetricsHandler_UpdateHandler(t *testing.T) {
	storage := repository.NewMemStorage()
	h := &MetricsHandler{Storage: storage}

	r := chi.NewRouter()
	r.Post("/update/{type}/{name}/{value}", h.UpdateHandler)

	// тестовый сервер
	ts := httptest.NewServer(r)
	defer ts.Close()

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
		{
			name:   "Неизвестный тип метрики",
			method: http.MethodPost,
			url:    "/update/unknown/testGauge/100",
			want:   want{code: http.StatusBadRequest},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, _ := testRequest(t, ts, tt.method, tt.url)
			defer resp.Body.Close()
			assert.Equal(t, tt.want.code, resp.StatusCode)
		})
	}
}

func TestMetricsHandler_ValueHandler(t *testing.T) {
	storage := repository.NewMemStorage()
	storage.UpdateGauge("existingGauge", 123.45)
	storage.UpdateCounter("existingCounter", 100)

	h := &MetricsHandler{Storage: storage}

	r := chi.NewRouter()
	r.Get("/value/{type}/{name}", h.ValueHandler)

	ts := httptest.NewServer(r)
	defer ts.Close()

	tests := []struct {
		name     string
		url      string
		wantCode int
		wantBody string
	}{
		{
			name:     "Получение существующего gauge",
			url:      "/value/gauge/existingGauge",
			wantCode: http.StatusOK,
			wantBody: "123.45",
		},
		{
			name:     "Получение существующего counter",
			url:      "/value/counter/existingCounter",
			wantCode: http.StatusOK,
			wantBody: "100",
		},
		{
			name:     "Получение несуществующей метрики",
			url:      "/value/gauge/nonExistent",
			wantCode: http.StatusNotFound,
			wantBody: "Метрика не найдена\n",
		},
		{
			name:     "Неверный тип метрики",
			url:      "/value/unknown/existingGauge",
			wantCode: http.StatusNotFound,
			wantBody: "Неверный тип метрики\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, body := testRequest(t, ts, http.MethodGet, tt.url)
			defer resp.Body.Close()
			assert.Equal(t, tt.wantCode, resp.StatusCode)
			assert.Equal(t, tt.wantBody, body)
		})
	}
}

func TestMetricsHandler_RootHandler(t *testing.T) {
	storage := repository.NewMemStorage()
	storage.UpdateCounter("testCounter", 5)
	h := &MetricsHandler{Storage: storage}

	r := chi.NewRouter()
	r.Get("/", h.RootHandler)

	ts := httptest.NewServer(r)
	defer ts.Close()

	resp, body := testRequest(t, ts, http.MethodGet, "/")
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "text/html", resp.Header.Get("Content-Type"))
	assert.Contains(t, body, "testCounter")
	assert.Contains(t, body, "5")
}
