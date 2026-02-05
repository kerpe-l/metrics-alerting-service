package handler

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/kerpe-l/metrics-alerting-service/internal/model"
	"github.com/kerpe-l/metrics-alerting-service/internal/repository"
)

type MetricsHandler struct {
	Storage repository.Storage
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
