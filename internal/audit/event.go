// Package audit реализует паттерн «Наблюдатель» для событий обработки метрик.
// Publisher рассылает события зарегистрированным наблюдателям (например, файловому и HTTP-приёмникам).
package audit

// Event описывает одно событие аудита, формируемое после успешной обработки метрик сервером.
type Event struct {
	// Timestamp — момент события в формате Unix timestamp (секунды).
	Timestamp int64 `json:"ts"`
	// Metrics — имена метрик, обработанных в рамках запроса.
	Metrics []string `json:"metrics"`
	// IPAddress — IP-адрес клиента, приславшего запрос.
	IPAddress string `json:"ip_address"`
}
