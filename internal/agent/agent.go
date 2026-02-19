package agent

import (
	"bytes"
	"encoding/json"
	"math/rand"
	"net/http"
	"runtime"

	"go.uber.org/zap"

	"github.com/kerpe-l/metrics-alerting-service/internal/logger"
	"github.com/kerpe-l/metrics-alerting-service/internal/model"
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

func (s *Stats) Send(serverAddr string) {
	url := serverAddr + "/update/"

	// 1. Отправляем все Gauge метрики
	for name, value := range s.RuntimeMetrics {
		val := value
		s.sendJSON(url, model.Metrics{ID: name, MType: model.Gauge, Value: &val})
	}

	// 2. Отправляем RandomValue
	rv := s.RandomValue
	s.sendJSON(url, model.Metrics{ID: "RandomValue", MType: model.Gauge, Value: &rv})

	// 3. Отправляем PollCount
	pc := s.PollCount
	s.sendJSON(url, model.Metrics{ID: "PollCount", MType: model.Counter, Delta: &pc})
}

func (s *Stats) sendJSON(url string, metric model.Metrics) {
	body, err := json.Marshal(metric)
	if err != nil {
		logger.Log.Info("failed to marshal metric", zap.Error(err))
		return
	}

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(body))
	if err != nil {
		logger.Log.Info("failed to send metric", zap.Error(err))
		return
	}
	resp.Body.Close()
}
