package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/kerpe-l/metrics-alerting-service/internal/audit"
	"github.com/kerpe-l/metrics-alerting-service/internal/logger"
	"github.com/kerpe-l/metrics-alerting-service/internal/model"
	"github.com/kerpe-l/metrics-alerting-service/internal/service"
)

// MetricsHandler — HTTP-обработчики работы с метриками.
type MetricsHandler struct {
	Service service.MetricsService
	Audit   *audit.Publisher
}

// publishAudit формирует и публикует событие аудита для запроса.
func (h *MetricsHandler) publishAudit(r *http.Request, names []string) {
	if h.Audit == nil || len(names) == 0 {
		return
	}
	h.Audit.Publish(audit.Event{
		Timestamp: time.Now().Unix(),
		Metrics:   names,
		IPAddress: clientIP(r),
	})
}

// clientIP возвращает IP-адрес клиента, отбрасывая порт у r.RemoteAddr.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func (h *MetricsHandler) UpdateHandler(w http.ResponseWriter, r *http.Request) {
	mType := chi.URLParam(r, "type")
	mName := chi.URLParam(r, "name")
	mValue := chi.URLParam(r, "value")

	var value float64
	var delta int64

	switch mType {
	case model.Gauge:
		v, err := strconv.ParseFloat(mValue, 64)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}
		value = v
	case model.Counter:
		v, err := strconv.ParseInt(mValue, 10, 64)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}
		delta = v
	default:
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	if err := h.Service.Update(r.Context(), mName, mType, value, delta); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.publishAudit(r, []string{mName})
	w.WriteHeader(http.StatusOK)
}

func (h *MetricsHandler) ValueHandler(w http.ResponseWriter, r *http.Request) {
	mType := chi.URLParam(r, "type")
	mName := chi.URLParam(r, "name")

	result, err := h.Service.GetValue(r.Context(), mName, mType)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	if _, err := w.Write([]byte(result)); err != nil {
		logger.Log.Error("ошибка записи ответа: " + err.Error())
	}
}

func (h *MetricsHandler) UpdateJSONHandler(w http.ResponseWriter, r *http.Request) {
	var metric model.Metrics

	if err := json.NewDecoder(r.Body).Decode(&metric); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	updated, err := h.Service.UpdateJSON(r.Context(), metric)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	h.publishAudit(r, []string{updated.ID})
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(updated); err != nil {
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

	result, err := h.Service.GetJSON(r.Context(), metric)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(result); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// UpdateBatchHandler принимает массив метрик и обновляет их одним батчем.
func (h *MetricsHandler) UpdateBatchHandler(w http.ResponseWriter, r *http.Request) {
	var metrics []model.Metrics

	if err := json.NewDecoder(r.Body).Decode(&metrics); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := h.Service.UpdateBatch(r.Context(), metrics); err != nil {
		writeServiceError(w, err)
		return
	}

	names := make([]string, 0, len(metrics))
	for _, m := range metrics {
		names = append(names, m.ID)
	}
	h.publishAudit(r, names)
	w.WriteHeader(http.StatusOK)
}

// PingDB проверяет соединение с базой данных.
func (h *MetricsHandler) PingDB(w http.ResponseWriter, r *http.Request) {
	if err := h.Service.Ping(r.Context()); err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// RootHandler отдает HTML со списком всех метрик
func (h *MetricsHandler) RootHandler(w http.ResponseWriter, r *http.Request) {
	gauges, counters := h.Service.GetAll(r.Context())

	w.Header().Set("Content-Type", "text/html")
	html := "<html><body><h1>Metrics</h1><ul>"

	for name, val := range gauges {
		html += fmt.Sprintf("<li>[Gauge] %s: %v</li>", name, val)
	}
	for name, val := range counters {
		html += fmt.Sprintf("<li>[Counter] %s: %v</li>", name, val)
	}

	html += "</ul></body></html>"
	if _, err := w.Write([]byte(html)); err != nil {
		logger.Log.Error("ошибка записи ответа: " + err.Error())
	}
}

// writeServiceError маппит ошибки сервиса на HTTP статус-коды.
func writeServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrMetricNotFound):
		http.Error(w, err.Error(), http.StatusNotFound)
	case errors.Is(err, service.ErrEmptyID):
		http.Error(w, err.Error(), http.StatusNotFound)
	case errors.Is(err, service.ErrInvalidType),
		errors.Is(err, service.ErrMissingValue),
		errors.Is(err, service.ErrMissingDelta),
		errors.Is(err, service.ErrEmptyBatch):
		http.Error(w, err.Error(), http.StatusBadRequest)
	default:
		logger.Log.Error("internal server error: " + err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
