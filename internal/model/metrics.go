// Package model описывает доменную модель метрик, общую для сервера и агента.
package model

// Типы метрик, используемые в поле Metrics.MType.
const (
	// Counter — счётчик: значения суммируются (накопительная метрика).
	Counter = "counter"
	// Gauge — измеритель: новое значение замещает предыдущее.
	Gauge = "gauge"
)

// Metrics описывает одну метрику в плоской JSON-модели.
//
// Delta и Value — указатели, чтобы отличать незаданное значение от нуля
// (отсутствующее поле не кодируется в JSON). Иерархическая вложенность
// сознательно не вводится, чтобы не усложнять модель.
type Metrics struct {
	// ID — имя метрики.
	ID string `json:"id"`
	// MType — тип метрики: model.Counter или model.Gauge.
	MType string `json:"type"`
	// Delta — значение для counter (накапливается). nil для gauge.
	Delta *int64 `json:"delta,omitempty"`
	// Value — значение для gauge (замещается). nil для counter.
	Value *float64 `json:"value,omitempty"`
	// Hash — HMAC-SHA256 подпись метрики (опционально).
	Hash string `json:"hash,omitempty"`
}
