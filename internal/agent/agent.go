package agent

import (
	"fmt"
	"math/rand"
	"net/http"
	"runtime"
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
	// 1. Отправляем все Gauge метрики
	for name, value := range s.RuntimeMetrics {
		url := fmt.Sprintf("%s/update/gauge/%s/%f", serverAddr, name, value)
		s.sendRequest(url)
	}

	// 2. Отправляем RandomValue
	urlRandom := fmt.Sprintf("%s/update/gauge/RandomValue/%f", serverAddr, s.RandomValue)
	s.sendRequest(urlRandom)

	// 3. Отправляем PollCount
	urlCount := fmt.Sprintf("%s/update/counter/PollCount/%d", serverAddr, s.PollCount)
	s.sendRequest(urlCount)
}

func (s *Stats) sendRequest(url string) {
	resp, err := http.Post(url, "text/plain", nil)
	if err != nil {
		fmt.Printf("Ошибка при отправке: %v\n", err)
		return
	}
	resp.Body.Close()
}
