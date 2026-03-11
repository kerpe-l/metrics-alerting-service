package agent

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"runtime"

	"go.uber.org/zap"

	"github.com/kerpe-l/metrics-alerting-service/internal/logger"
	"github.com/kerpe-l/metrics-alerting-service/internal/model"
	"github.com/kerpe-l/metrics-alerting-service/internal/retry"
)

type Stats struct {
	RuntimeMetrics map[string]float64
	PollCount      int64
	RandomValue    float64
}

func NewStats() *Stats {
	return &Stats{
		RuntimeMetrics: make(map[string]float64),
	}
}

// Collect собирает данные из runtime и обновляет внутренние поля
func (s *Stats) Collect() {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	// gauge
	s.RuntimeMetrics["Alloc"] = float64(mem.Alloc)
	s.RuntimeMetrics["BuckHashSys"] = float64(mem.BuckHashSys)
	s.RuntimeMetrics["Frees"] = float64(mem.Frees)
	s.RuntimeMetrics["GCCPUFraction"] = mem.GCCPUFraction
	s.RuntimeMetrics["GCSys"] = float64(mem.GCSys)
	s.RuntimeMetrics["HeapAlloc"] = float64(mem.HeapAlloc)
	s.RuntimeMetrics["HeapIdle"] = float64(mem.HeapIdle)
	s.RuntimeMetrics["HeapInuse"] = float64(mem.HeapInuse)
	s.RuntimeMetrics["HeapObjects"] = float64(mem.HeapObjects)
	s.RuntimeMetrics["HeapReleased"] = float64(mem.HeapReleased)
	s.RuntimeMetrics["HeapSys"] = float64(mem.HeapSys)
	s.RuntimeMetrics["LastGC"] = float64(mem.LastGC)
	s.RuntimeMetrics["Lookups"] = float64(mem.Lookups)
	s.RuntimeMetrics["MCacheInuse"] = float64(mem.MCacheInuse)
	s.RuntimeMetrics["MCacheSys"] = float64(mem.MCacheSys)
	s.RuntimeMetrics["MSpanInuse"] = float64(mem.MSpanInuse)
	s.RuntimeMetrics["MSpanSys"] = float64(mem.MSpanSys)
	s.RuntimeMetrics["Mallocs"] = float64(mem.Mallocs)
	s.RuntimeMetrics["NextGC"] = float64(mem.NextGC)
	s.RuntimeMetrics["NumForcedGC"] = float64(mem.NumForcedGC)
	s.RuntimeMetrics["NumGC"] = float64(mem.NumGC)
	s.RuntimeMetrics["OtherSys"] = float64(mem.OtherSys)
	s.RuntimeMetrics["PauseTotalNs"] = float64(mem.PauseTotalNs)
	s.RuntimeMetrics["StackInuse"] = float64(mem.StackInuse)
	s.RuntimeMetrics["StackSys"] = float64(mem.StackSys)
	s.RuntimeMetrics["Sys"] = float64(mem.Sys)
	s.RuntimeMetrics["TotalAlloc"] = float64(mem.TotalAlloc)

	s.PollCount++
	s.RandomValue = rand.Float64()
}

// SendBatch отправляет все метрики одним запросом на /updates/.
func (s *Stats) SendBatch(serverAddr string) {
	metrics := make([]model.Metrics, 0, len(s.RuntimeMetrics)+2)

	// 1. Отправляем все Gauge метрики
	for name, value := range s.RuntimeMetrics {
		val := value
		metrics = append(metrics, model.Metrics{ID: name, MType: model.Gauge, Value: &val})
	}

	// 2. Отправляем RandomValue
	rv := s.RandomValue
	metrics = append(metrics, model.Metrics{ID: "RandomValue", MType: model.Gauge, Value: &rv})

	// 3. Отправляем PollCount
	pc := s.PollCount
	metrics = append(metrics, model.Metrics{ID: "PollCount", MType: model.Counter, Delta: &pc})

	if len(metrics) == 0 {
		return
	}

	s.sendJSON(serverAddr+"/updates/", metrics)
}

func (s *Stats) sendJSON(url string, body any) {
	data, err := json.Marshal(body)
	if err != nil {
		logger.Log.Info("failed to marshal", zap.Error(err))
		return
	}

	// Сжимаем запрос через gzip
	var buf bytes.Buffer
	gz, err := gzip.NewWriterLevel(&buf, gzip.BestSpeed)
	if err != nil {
		logger.Log.Info("failed to create gzip writer", zap.Error(err))
		return
	}
	_, err = gz.Write(data)
	if err != nil {
		logger.Log.Info("failed to write gzip data", zap.Error(err))
		return
	}
	gz.Close()

	compressed := buf.Bytes()

	err = retry.Do(func() error {
		req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(compressed))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Content-Encoding", "gzip")
		req.Header.Set("Accept-Encoding", "gzip")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		if resp.StatusCode >= http.StatusInternalServerError {
			return fmt.Errorf("server returned %d", resp.StatusCode)
		}
		return nil
	}, func(err error) bool {
		// Любая сетевая ошибка или 5xx — retriable
		return true
	})
	if err != nil {
		logger.Log.Info("failed to send metrics", zap.Error(err))
	}
}
