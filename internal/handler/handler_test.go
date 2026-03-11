package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/kerpe-l/metrics-alerting-service/internal/model"
	"github.com/kerpe-l/metrics-alerting-service/internal/repository"
	"github.com/kerpe-l/metrics-alerting-service/internal/repository/mock"
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
			name:   "Некорректное значение (gauge)",
			method: http.MethodPost,
			url:    "/update/gauge/testGauge/none",
			want:   want{code: http.StatusBadRequest},
		},
		{
			name:   "Некорректное значение (counter)",
			method: http.MethodPost,
			url:    "/update/counter/testCounter/none",
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
			wantBody: http.StatusText(http.StatusNotFound) + "\n",
		},
		{
			name:     "Неверный тип метрики",
			url:      "/value/unknown/existingGauge",
			wantCode: http.StatusBadRequest,
			wantBody: http.StatusText(http.StatusBadRequest) + "\n",
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

func TestMetricsHandler_PingDB(t *testing.T) {
	tests := []struct {
		name     string
		handler  *MetricsHandler
		wantCode int
	}{
		{
			name:     "DB не задана — 500",
			handler:  &MetricsHandler{Storage: repository.NewMemStorage(), DB: nil},
			wantCode: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := chi.NewRouter()
			r.Get("/ping", tt.handler.PingDB)

			ts := httptest.NewServer(r)
			defer ts.Close()

			resp, _ := testRequest(t, ts, http.MethodGet, "/ping")
			defer resp.Body.Close()
			assert.Equal(t, tt.wantCode, resp.StatusCode)
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

func ptrFloat64(v float64) *float64 { return &v }
func ptrInt64(v int64) *int64       { return &v }

func testJSONRequest(t *testing.T, ts *httptest.Server, method, path string, body interface{}) (*http.Response, string) {
	data, err := json.Marshal(body)
	require.NoError(t, err)

	req, err := http.NewRequest(method, ts.URL+path, bytes.NewBuffer(data))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := ts.Client().Do(req)
	require.NoError(t, err)

	respBody, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	return resp, string(respBody)
}

func TestMetricsHandler_UpdateJSONHandler(t *testing.T) {
	storage := repository.NewMemStorage()
	h := &MetricsHandler{Storage: storage}

	r := chi.NewRouter()
	r.Post("/update/", h.UpdateJSONHandler)

	ts := httptest.NewServer(r)
	defer ts.Close()

	tests := []struct {
		name      string
		payload   model.Metrics
		wantCode  int
		wantValue *float64
		wantDelta *int64
	}{
		{
			name:      "gauge update",
			payload:   model.Metrics{ID: "testGauge", MType: model.Gauge, Value: ptrFloat64(100.5)},
			wantCode:  http.StatusOK,
			wantValue: ptrFloat64(100.5),
		},
		{
			name:      "counter update",
			payload:   model.Metrics{ID: "testCounter", MType: model.Counter, Delta: ptrInt64(10)},
			wantCode:  http.StatusOK,
			wantDelta: ptrInt64(10),
		},
		{
			name:     "empty ID",
			payload:  model.Metrics{ID: "", MType: model.Gauge, Value: ptrFloat64(1.0)},
			wantCode: http.StatusNotFound,
		},
		{
			name:     "unknown type",
			payload:  model.Metrics{ID: "test", MType: "unknown", Value: ptrFloat64(1.0)},
			wantCode: http.StatusBadRequest,
		},
		{
			name:     "nil value for gauge",
			payload:  model.Metrics{ID: "test", MType: model.Gauge},
			wantCode: http.StatusBadRequest,
		},
		{
			name:     "nil delta for counter",
			payload:  model.Metrics{ID: "test", MType: model.Counter},
			wantCode: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, body := testJSONRequest(t, ts, http.MethodPost, "/update/", tt.payload)
			defer resp.Body.Close()

			assert.Equal(t, tt.wantCode, resp.StatusCode)

			if tt.wantCode == http.StatusOK {
				assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))
				var result model.Metrics
				require.NoError(t, json.Unmarshal([]byte(body), &result))
				if tt.wantValue != nil {
					require.NotNil(t, result.Value)
					assert.InDelta(t, *tt.wantValue, *result.Value, 0.001)
				}
				if tt.wantDelta != nil {
					require.NotNil(t, result.Delta)
					assert.Equal(t, *tt.wantDelta, *result.Delta)
				}
			}
		})
	}

	t.Run("counter accumulation", func(t *testing.T) {
		resp1, _ := testJSONRequest(t, ts, http.MethodPost, "/update/",
			model.Metrics{ID: "accCounter", MType: model.Counter, Delta: ptrInt64(5)})
		resp1.Body.Close()
		resp, body := testJSONRequest(t, ts, http.MethodPost, "/update/",
			model.Metrics{ID: "accCounter", MType: model.Counter, Delta: ptrInt64(3)})
		defer resp.Body.Close()

		var result model.Metrics
		require.NoError(t, json.Unmarshal([]byte(body), &result))
		require.NotNil(t, result.Delta)
		assert.Equal(t, int64(8), *result.Delta)
	})

	t.Run("malformed JSON", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodPost, ts.URL+"/update/", bytes.NewBufferString("not json"))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")

		resp, err := ts.Client().Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})
}

func TestMetricsHandler_ValueJSONHandler(t *testing.T) {
	storage := repository.NewMemStorage()
	storage.UpdateGauge("existingGauge", 123.45)
	storage.UpdateCounter("existingCounter", 100)

	h := &MetricsHandler{Storage: storage}

	r := chi.NewRouter()
	r.Post("/value/", h.ValueJSONHandler)

	ts := httptest.NewServer(r)
	defer ts.Close()

	tests := []struct {
		name      string
		payload   model.Metrics
		wantCode  int
		wantValue *float64
		wantDelta *int64
	}{
		{
			name:      "get existing gauge",
			payload:   model.Metrics{ID: "existingGauge", MType: model.Gauge},
			wantCode:  http.StatusOK,
			wantValue: ptrFloat64(123.45),
		},
		{
			name:      "get existing counter",
			payload:   model.Metrics{ID: "existingCounter", MType: model.Counter},
			wantCode:  http.StatusOK,
			wantDelta: ptrInt64(100),
		},
		{
			name:     "non-existent metric",
			payload:  model.Metrics{ID: "noSuchMetric", MType: model.Gauge},
			wantCode: http.StatusNotFound,
		},
		{
			name:     "empty ID",
			payload:  model.Metrics{ID: "", MType: model.Gauge},
			wantCode: http.StatusNotFound,
		},
		{
			name:     "unknown type",
			payload:  model.Metrics{ID: "existingGauge", MType: "invalid"},
			wantCode: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, body := testJSONRequest(t, ts, http.MethodPost, "/value/", tt.payload)
			defer resp.Body.Close()

			assert.Equal(t, tt.wantCode, resp.StatusCode)

			if tt.wantCode == http.StatusOK {
				assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))
				var result model.Metrics
				require.NoError(t, json.Unmarshal([]byte(body), &result))
				if tt.wantValue != nil {
					require.NotNil(t, result.Value)
					assert.InDelta(t, *tt.wantValue, *result.Value, 0.001)
				}
				if tt.wantDelta != nil {
					require.NotNil(t, result.Delta)
					assert.Equal(t, *tt.wantDelta, *result.Delta)
				}
			}
		})
	}
}

// --- Тесты с MockStorage ---

func TestUpdateBatchHandler_MockStorage(t *testing.T) {
	tests := []struct {
		name     string
		body     any
		prepare  func(s *mock.MockStorage)
		wantCode int
	}{
		{
			name: "Успешный батч gauge + counter",
			body: []model.Metrics{
				{ID: "cpu", MType: model.Gauge, Value: ptrFloat64(42.5)},
				{ID: "hits", MType: model.Counter, Delta: ptrInt64(10)},
			},
			prepare: func(s *mock.MockStorage) {
				s.EXPECT().UpdateBatch(gomock.Any()).Return(nil)
			},
			wantCode: http.StatusOK,
		},
		{
			name:     "Пустой батч — 400",
			body:     []model.Metrics{},
			prepare:  func(s *mock.MockStorage) {},
			wantCode: http.StatusBadRequest,
		},
		{
			name: "Ошибка хранилища — 500",
			body: []model.Metrics{
				{ID: "cpu", MType: model.Gauge, Value: ptrFloat64(1.0)},
			},
			prepare: func(s *mock.MockStorage) {
				s.EXPECT().UpdateBatch(gomock.Any()).Return(errors.New("db connection lost"))
			},
			wantCode: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			storage := mock.NewMockStorage(ctrl)
			tt.prepare(storage)

			h := &MetricsHandler{Storage: storage}
			r := chi.NewRouter()
			r.Post("/updates/", h.UpdateBatchHandler)

			ts := httptest.NewServer(r)
			defer ts.Close()

			resp, _ := testJSONRequest(t, ts, http.MethodPost, "/updates/", tt.body)
			defer resp.Body.Close()

			assert.Equal(t, tt.wantCode, resp.StatusCode)
		})
	}

	t.Run("Невалидный JSON — 400", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		storage := mock.NewMockStorage(ctrl)
		h := &MetricsHandler{Storage: storage}

		r := chi.NewRouter()
		r.Post("/updates/", h.UpdateBatchHandler)

		ts := httptest.NewServer(r)
		defer ts.Close()

		req, err := http.NewRequest(http.MethodPost, ts.URL+"/updates/", bytes.NewBufferString("not json"))
		require.NoError(t, err)

		resp, err := ts.Client().Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})
}

func TestUpdateJSONHandler_GetAfterUpdateFails(t *testing.T) {
	tests := []struct {
		name     string
		payload  model.Metrics
		prepare  func(s *mock.MockStorage)
		wantCode int
	}{
		{
			name:    "GetGauge не нашёл после обновления — 500",
			payload: model.Metrics{ID: "ghost", MType: model.Gauge, Value: ptrFloat64(1.0)},
			prepare: func(s *mock.MockStorage) {
				s.EXPECT().UpdateGauge("ghost", 1.0)
				s.EXPECT().GetGauge("ghost").Return(0.0, false)
			},
			wantCode: http.StatusInternalServerError,
		},
		{
			name:    "GetCounter не нашёл после обновления — 500",
			payload: model.Metrics{ID: "ghost", MType: model.Counter, Delta: ptrInt64(1)},
			prepare: func(s *mock.MockStorage) {
				s.EXPECT().UpdateCounter("ghost", int64(1))
				s.EXPECT().GetCounter("ghost").Return(int64(0), false)
			},
			wantCode: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			storage := mock.NewMockStorage(ctrl)
			tt.prepare(storage)

			h := &MetricsHandler{Storage: storage}
			r := chi.NewRouter()
			r.Post("/update/", h.UpdateJSONHandler)

			ts := httptest.NewServer(r)
			defer ts.Close()

			resp, _ := testJSONRequest(t, ts, http.MethodPost, "/update/", tt.payload)
			defer resp.Body.Close()

			assert.Equal(t, tt.wantCode, resp.StatusCode)
		})
	}
}
