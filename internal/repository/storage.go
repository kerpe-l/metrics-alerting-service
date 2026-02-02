package repository

type Storage interface {
	UpdateGauge(name string, value float64)
	UpdateCounter(name string, value int64)
	GetGauge(name string) (float64, bool)
	GetCounter(name string) (int64, bool)
	GetAll() (map[string]float64, map[string]int64)
}

type MemStorage struct {
	gauges   map[string]float64
	counters map[string]int64
}

func NewMemStorage() *MemStorage {
	return &MemStorage{
		gauges:   make(map[string]float64),
		counters: make(map[string]int64),
	}
}

func (ms *MemStorage) UpdateGauge(name string, value float64) {
	ms.gauges[name] = value
}

func (ms *MemStorage) UpdateCounter(name string, value int64) {
	ms.counters[name] += value
}

func (ms *MemStorage) GetGauge(name string) (float64, bool) {
	v, ok := ms.gauges[name]
	return v, ok
}

func (ms *MemStorage) GetCounter(name string) (int64, bool) {
	v, ok := ms.counters[name]
	return v, ok
}

func (ms *MemStorage) GetAll() (map[string]float64, map[string]int64) {
	return ms.gauges, ms.counters
}
