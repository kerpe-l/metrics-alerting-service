// Package agent реализует агент сбора метрик: периодический опрос
// runtime- и системных метрик (Collector) и их батч-отправку на сервер (Sender).
package agent

import (
	"fmt"
	"math/rand"
	"runtime"
	"sync"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/mem"

	"github.com/kerpe-l/metrics-alerting-service/internal/logger"
	"github.com/kerpe-l/metrics-alerting-service/internal/model"
	"go.uber.org/zap"
)

// Collector собирает runtime-метрики.
type Collector struct {
	mu             sync.RWMutex
	runtimeMetrics map[string]float64
	pollCount      int64
	randomValue    float64
	extraMetrics   map[string]float64
}

// NewCollector создаёт новый коллектор метрик.
func NewCollector() *Collector {
	return &Collector{
		runtimeMetrics: make(map[string]float64),
		extraMetrics:   make(map[string]float64),
	}
}

// Collect считывает метрики из runtime.MemStats.
func (c *Collector) Collect() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	c.mu.Lock()
	defer c.mu.Unlock()

	c.runtimeMetrics["Alloc"] = float64(m.Alloc)
	c.runtimeMetrics["BuckHashSys"] = float64(m.BuckHashSys)
	c.runtimeMetrics["Frees"] = float64(m.Frees)
	c.runtimeMetrics["GCCPUFraction"] = m.GCCPUFraction
	c.runtimeMetrics["GCSys"] = float64(m.GCSys)
	c.runtimeMetrics["HeapAlloc"] = float64(m.HeapAlloc)
	c.runtimeMetrics["HeapIdle"] = float64(m.HeapIdle)
	c.runtimeMetrics["HeapInuse"] = float64(m.HeapInuse)
	c.runtimeMetrics["HeapObjects"] = float64(m.HeapObjects)
	c.runtimeMetrics["HeapReleased"] = float64(m.HeapReleased)
	c.runtimeMetrics["HeapSys"] = float64(m.HeapSys)
	c.runtimeMetrics["LastGC"] = float64(m.LastGC)
	c.runtimeMetrics["Lookups"] = float64(m.Lookups)
	c.runtimeMetrics["MCacheInuse"] = float64(m.MCacheInuse)
	c.runtimeMetrics["MCacheSys"] = float64(m.MCacheSys)
	c.runtimeMetrics["MSpanInuse"] = float64(m.MSpanInuse)
	c.runtimeMetrics["MSpanSys"] = float64(m.MSpanSys)
	c.runtimeMetrics["Mallocs"] = float64(m.Mallocs)
	c.runtimeMetrics["NextGC"] = float64(m.NextGC)
	c.runtimeMetrics["NumForcedGC"] = float64(m.NumForcedGC)
	c.runtimeMetrics["NumGC"] = float64(m.NumGC)
	c.runtimeMetrics["OtherSys"] = float64(m.OtherSys)
	c.runtimeMetrics["PauseTotalNs"] = float64(m.PauseTotalNs)
	c.runtimeMetrics["StackInuse"] = float64(m.StackInuse)
	c.runtimeMetrics["StackSys"] = float64(m.StackSys)
	c.runtimeMetrics["Sys"] = float64(m.Sys)
	c.runtimeMetrics["TotalAlloc"] = float64(m.TotalAlloc)

	c.pollCount++
	c.randomValue = rand.Float64()
}

// CollectExtra собирает дополнительные метрики через gopsutil.
func (c *Collector) CollectExtra() {
	extra := make(map[string]float64)

	v, err := mem.VirtualMemory()
	if err != nil {
		logger.Log.Error("failed to get virtual memory", zap.Error(err))
	} else {
		extra["TotalMemory"] = float64(v.Total)
		extra["FreeMemory"] = float64(v.Free)
	}

	cpuPercent, err := cpu.Percent(0, true)
	if err != nil {
		logger.Log.Error("failed to get cpu percent", zap.Error(err))
	} else {
		for i, pct := range cpuPercent {
			extra[fmt.Sprintf("CPUutilization%d", i+1)] = pct
		}
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.extraMetrics = extra
}

// Metrics формирует слайс метрик для отправки.
func (c *Collector) Metrics() []model.Metrics {
	c.mu.RLock()
	defer c.mu.RUnlock()

	metrics := make([]model.Metrics, 0, len(c.runtimeMetrics)+len(c.extraMetrics)+2)

	for name, value := range c.runtimeMetrics {
		val := value
		metrics = append(metrics, model.Metrics{ID: name, MType: model.Gauge, Value: &val})
	}

	for name, value := range c.extraMetrics {
		val := value
		metrics = append(metrics, model.Metrics{ID: name, MType: model.Gauge, Value: &val})
	}

	rv := c.randomValue
	metrics = append(metrics, model.Metrics{ID: "RandomValue", MType: model.Gauge, Value: &rv})

	pc := c.pollCount
	metrics = append(metrics, model.Metrics{ID: "PollCount", MType: model.Counter, Delta: &pc})

	return metrics
}
