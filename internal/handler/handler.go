package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kerpe-l/metrics-alerting-service/internal/model"
	"github.com/kerpe-l/metrics-alerting-service/internal/repository"
)

type MetricsHandler struct {
	Storage repository.Storage
	DB      *pgxpool.Pool
}

func (h *MetricsHandler) UpdateHandler(w http.ResponseWriter, r *http.Request) {
	mType := chi.URLParam(r, "type")
	mName := chi.URLParam(r, "name")
	mValue := chi.URLParam(r, "value")

	switch mType {
	case model.Gauge:
		val, err := strconv.ParseFloat(mValue, 64)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}
		h.Storage.UpdateGauge(mName, val)

	case model.Counter:
		val, err := strconv.ParseInt(mValue, 10, 64)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}
		h.Storage.UpdateCounter(mName, val)

	default:
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (h *MetricsHandler) ValueHandler(w http.ResponseWriter, r *http.Request) {
	mType := chi.URLParam(r, "type")
	mName := chi.URLParam(r, "name")

	switch mType {
	case model.Gauge:
		val, ok := h.Storage.GetGauge(mName)
		if !ok {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}
		w.Write([]byte(strconv.FormatFloat(val, 'f', -1, 64)))

	case model.Counter:
		val, ok := h.Storage.GetCounter(mName)
		if !ok {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}
		w.Write([]byte(strconv.FormatInt(val, 10)))

	default:
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
}

func (h *MetricsHandler) UpdateJSONHandler(w http.ResponseWriter, r *http.Request) {
	var metric model.Metrics

	if err := json.NewDecoder(r.Body).Decode(&metric); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if metric.ID == "" {
		http.Error(w, "metric ID is required", http.StatusNotFound)
		return
	}

	switch metric.MType {
	case model.Gauge:
		if metric.Value == nil {
			http.Error(w, "value is required for gauge", http.StatusBadRequest)
			return
		}
		h.Storage.UpdateGauge(metric.ID, *metric.Value)
		val, ok := h.Storage.GetGauge(metric.ID)
		if !ok {
			http.Error(w, "failed to get gauge after update", http.StatusInternalServerError)
			return
		}
		metric.Value = &val

	case model.Counter:
		if metric.Delta == nil {
			http.Error(w, "delta is required for counter", http.StatusBadRequest)
			return
		}
		h.Storage.UpdateCounter(metric.ID, *metric.Delta)
		val, ok := h.Storage.GetCounter(metric.ID)
		if !ok {
			http.Error(w, "failed to get counter after update", http.StatusInternalServerError)
			return
		}
		metric.Delta = &val

	default:
		http.Error(w, "unknown metric type", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(metric); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (h *MetricsHandler) ValueJSONHandler(w http.ResponseWriter, r *http.Request) {
	var metric model.Metrics

	if err := json.NewDecoder(r.Body).Decode(&metric); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if metric.ID == "" {
		http.Error(w, "metric ID is required", http.StatusNotFound)
		return
	}

	switch metric.MType {
	case model.Gauge:
		val, ok := h.Storage.GetGauge(metric.ID)
		if !ok {
			http.Error(w, "metric not found", http.StatusNotFound)
			return
		}
		metric.Value = &val

	case model.Counter:
		val, ok := h.Storage.GetCounter(metric.ID)
		if !ok {
			http.Error(w, "metric not found", http.StatusNotFound)
			return
		}
		metric.Delta = &val

	default:
		http.Error(w, "unknown metric type", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(metric); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// PingDB проверяет соединение с базой данных.
func (h *MetricsHandler) PingDB(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	if err := h.DB.Ping(r.Context()); err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// RootHandler отдает HTML со списком всех метрик
func (h *MetricsHandler) RootHandler(w http.ResponseWriter, r *http.Request) {
	gauges, counters := h.Storage.GetAll()

	w.Header().Set("Content-Type", "text/html")
	html := "<html><body><h1>Metrics</h1><ul>"

	for name, val := range gauges {
		html += fmt.Sprintf("<li>[Gauge] %s: %v</li>", name, val)
	}
	for name, val := range counters {
		html += fmt.Sprintf("<li>[Counter] %s: %v</li>", name, val)
	}

	html += "</ul></body></html>"
	w.Write([]byte(html))
}
