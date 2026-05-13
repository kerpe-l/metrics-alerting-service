package audit

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// defaultHTTPTimeout — таймаут на одну POST-доставку события.
const defaultHTTPTimeout = 5 * time.Second

// HTTPSink — приёмник событий аудита, отправляющий каждое событие методом POST на сконфигурированный URL с телом application/json.
type HTTPSink struct {
	url    string
	client *http.Client
}

// NewHTTPSink создаёт HTTPSink с http.Client с дефолтным таймаутом.
func NewHTTPSink(url string) *HTTPSink {
	return &HTTPSink{
		url:    url,
		client: &http.Client{Timeout: defaultHTTPTimeout},
	}
}

// Notify сериализует событие в JSON и отправляет POST-запросом.
// Статусы вне диапазона 2xx считаются ошибкой.
func (s *HTTPSink) Notify(ctx context.Context, ev Event) error {
	data, err := json.Marshal(ev)
	if err != nil {
		return fmt.Errorf("сериализация события: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.url, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("создание запроса: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("отправка события: %w", err)
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("неуспешный ответ приёмника аудита: %d", resp.StatusCode)
	}
	return nil
}
