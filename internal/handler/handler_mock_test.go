package handler

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/kerpe-l/metrics-alerting-service/internal/model"
	"github.com/kerpe-l/metrics-alerting-service/internal/repository/mock"
)

func TestUpdateBatchHandler_Mock(t *testing.T) {
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
			name: "Одна метрика в батче",
			body: []model.Metrics{
				{ID: "mem", MType: model.Gauge, Value: ptrFloat64(1024)},
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

func TestUpdateJSONHandler_Mock(t *testing.T) {
	tests := []struct {
		name     string
		payload  model.Metrics
		prepare  func(s *mock.MockStorage)
		wantCode int
	}{
		{
			name:    "Gauge — вызывает UpdateGauge и GetGauge",
			payload: model.Metrics{ID: "temp", MType: model.Gauge, Value: ptrFloat64(36.6)},
			prepare: func(s *mock.MockStorage) {
				s.EXPECT().UpdateGauge("temp", 36.6)
				s.EXPECT().GetGauge("temp").Return(36.6, true)
			},
			wantCode: http.StatusOK,
		},
		{
			name:    "Counter — вызывает UpdateCounter и GetCounter",
			payload: model.Metrics{ID: "reqs", MType: model.Counter, Delta: ptrInt64(5)},
			prepare: func(s *mock.MockStorage) {
				s.EXPECT().UpdateCounter("reqs", int64(5))
				s.EXPECT().GetCounter("reqs").Return(int64(5), true)
			},
			wantCode: http.StatusOK,
		},
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

func TestValueJSONHandler_Mock(t *testing.T) {
	tests := []struct {
		name     string
		payload  model.Metrics
		prepare  func(s *mock.MockStorage)
		wantCode int
	}{
		{
			name:    "Gauge найден",
			payload: model.Metrics{ID: "temp", MType: model.Gauge},
			prepare: func(s *mock.MockStorage) {
				s.EXPECT().GetGauge("temp").Return(36.6, true)
			},
			wantCode: http.StatusOK,
		},
		{
			name:    "Counter найден",
			payload: model.Metrics{ID: "reqs", MType: model.Counter},
			prepare: func(s *mock.MockStorage) {
				s.EXPECT().GetCounter("reqs").Return(int64(42), true)
			},
			wantCode: http.StatusOK,
		},
		{
			name:    "Gauge не найден — 404",
			payload: model.Metrics{ID: "missing", MType: model.Gauge},
			prepare: func(s *mock.MockStorage) {
				s.EXPECT().GetGauge("missing").Return(0.0, false)
			},
			wantCode: http.StatusNotFound,
		},
		{
			name:    "Counter не найден — 404",
			payload: model.Metrics{ID: "missing", MType: model.Counter},
			prepare: func(s *mock.MockStorage) {
				s.EXPECT().GetCounter("missing").Return(int64(0), false)
			},
			wantCode: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			storage := mock.NewMockStorage(ctrl)
			tt.prepare(storage)

			h := &MetricsHandler{Storage: storage}

			r := chi.NewRouter()
			r.Post("/value/", h.ValueJSONHandler)

			ts := httptest.NewServer(r)
			defer ts.Close()

			resp, _ := testJSONRequest(t, ts, http.MethodPost, "/value/", tt.payload)
			defer resp.Body.Close()

			assert.Equal(t, tt.wantCode, resp.StatusCode)
		})
	}
}

func TestRootHandler_Mock(t *testing.T) {
	ctrl := gomock.NewController(t)

	storage := mock.NewMockStorage(ctrl)
	storage.EXPECT().GetAll().Return(
		map[string]float64{"cpu": 55.5},
		map[string]int64{"hits": 100},
	)

	h := &MetricsHandler{Storage: storage}

	r := chi.NewRouter()
	r.Get("/", h.RootHandler)

	ts := httptest.NewServer(r)
	defer ts.Close()

	resp, body := testRequest(t, ts, http.MethodGet, "/")
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, body, "cpu")
	assert.Contains(t, body, "55.5")
	assert.Contains(t, body, "hits")
	assert.Contains(t, body, "100")
}
