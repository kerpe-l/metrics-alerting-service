package handler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/kerpe-l/metrics-alerting-service/internal/repository"
)

type MetricsHandler struct {
	Storage *repository.MemStorage
}

func (h *MetricsHandler) UpdateHandler(w http.ResponseWriter, r *http.Request) {
	// Проверяем, что метод POST
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	// Разбиваем URL на части
	// Пример: /update/gauge/Alloc/1.0 -> ["update", "gauge", "Alloc", "1.0"]
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")

	if len(parts) < 3 {
		http.Error(w, "Имя метрики не указано", http.StatusNotFound)
		return
	}
	if len(parts) < 4 {
		http.Error(w, "Значение метрики не указано", http.StatusBadRequest)
		return
	}

	mType := parts[1]
	mName := parts[2]
	mValue := parts[3]

	// Логика сохранения
	switch mType {
	case "gauge":
		val, err := strconv.ParseFloat(mValue, 64)
		if err != nil {
			http.Error(w, "Некорректное значение", http.StatusBadRequest)
			return
		}
		h.Storage.Gauges[mName] = val

	case "counter":
		val, err := strconv.ParseInt(mValue, 10, 64)
		if err != nil {
			http.Error(w, "Некорректное значение", http.StatusBadRequest)
			return
		}
		h.Storage.Counters[mName] += val

	default:
		http.Error(w, "Неверный тип метрики", http.StatusBadRequest)
		return
	}

	// Отправляем успешный статус
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
}
