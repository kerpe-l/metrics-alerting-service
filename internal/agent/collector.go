package agent

import (
	"math/rand"
	"runtime"

	"github.com/kerpe-l/metrics-alerting-service/internal/model"
)

// Collector собирает runtime-метрики.
type Collector struct {
	runtimeMetrics map[string]float64
	pollCount      int64
	randomValue    float64
}

// NewCollector создаёт новый коллектор метрик.
func NewCollector() *Collector {
	return &Collector{
		runtimeMetrics: make(map[string]float64),
	}
}

// Collect считывает метрики из runtime.MemStats.
func (c *Collector) Collect() {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	c.runtimeMetrics["Alloc"] = float64(mem.Alloc)
	c.runtimeMetrics["BuckHashSys"] = float64(mem.BuckHashSys)
	c.runtimeMetrics["Frees"] = float64(mem.Frees)
	c.runtimeMetrics["GCCPUFraction"] = mem.GCCPUFraction
	c.runtimeMetrics["GCSys"] = float64(mem.GCSys)
	c.runtimeMetrics["HeapAlloc"] = float64(mem.HeapAlloc)
	c.runtimeMetrics["HeapIdle"] = float64(mem.HeapIdle)
	c.runtimeMetrics["HeapInuse"] = float64(mem.HeapInuse)
	c.runtimeMetrics["HeapObjects"] = float64(mem.HeapObjects)
	c.runtimeMetrics["HeapReleased"] = float64(mem.HeapReleased)
	c.runtimeMetrics["HeapSys"] = float64(mem.HeapSys)
	c.runtimeMetrics["LastGC"] = float64(mem.LastGC)
	c.runtimeMetrics["Lookups"] = float64(mem.Lookups)
	c.runtimeMetrics["MCacheInuse"] = float64(mem.MCacheInuse)
	c.runtimeMetrics["MCacheSys"] = float64(mem.MCacheSys)
	c.runtimeMetrics["MSpanInuse"] = float64(mem.MSpanInuse)
	c.runtimeMetrics["MSpanSys"] = float64(mem.MSpanSys)
	c.runtimeMetrics["Mallocs"] = float64(mem.Mallocs)
	c.runtimeMetrics["NextGC"] = float64(mem.NextGC)
	c.runtimeMetrics["NumForcedGC"] = float64(mem.NumForcedGC)
	c.runtimeMetrics["NumGC"] = float64(mem.NumGC)
	c.runtimeMetrics["OtherSys"] = float64(mem.OtherSys)
	c.runtimeMetrics["PauseTotalNs"] = float64(mem.PauseTotalNs)
	c.runtimeMetrics["StackInuse"] = float64(mem.StackInuse)
	c.runtimeMetrics["StackSys"] = float64(mem.StackSys)
	c.runtimeMetrics["Sys"] = float64(mem.Sys)
	c.runtimeMetrics["TotalAlloc"] = float64(mem.TotalAlloc)

	c.pollCount++
	c.randomValue = rand.Float64()
}

// Metrics формирует слайс метрик для отправки.
func (c *Collector) Metrics() []model.Metrics {
	metrics := make([]model.Metrics, 0, len(c.runtimeMetrics)+2)

	for name, value := range c.runtimeMetrics {
		val := value
		metrics = append(metrics, model.Metrics{ID: name, MType: model.Gauge, Value: &val})
	}

	rv := c.randomValue
	metrics = append(metrics, model.Metrics{ID: "RandomValue", MType: model.Gauge, Value: &rv})

	pc := c.pollCount
	metrics = append(metrics, model.Metrics{ID: "PollCount", MType: model.Counter, Delta: &pc})

	return metrics
}
