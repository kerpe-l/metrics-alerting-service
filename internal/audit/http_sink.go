package audit

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/kerpe-l/metrics-alerting-service/internal/retry"
)

// defaultHTTPTimeout — таймаут на одну POST-доставку события.
const defaultHTTPTimeout = 5 * time.Second

// httpStatusError — ошибка неуспешного HTTP-статуса ответа приёмника.
// Хранит код, чтобы вызывающий код мог решать о повторе через errors.As.
type httpStatusError struct {
	code int
}

func (e *httpStatusError) Error() string {
	return fmt.Sprintf("unsuccessful audit sink response: %d", e.code)
}

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

// Notify сериализует событие в JSON и отправляет POST-запросом, повторяя
// попытку при временных сбоях (сетевые ошибки, статусы 5xx и 429) через
// retry.Do. Статусы вне диапазона 2xx считаются ошибкой.
func (s *HTTPSink) Notify(ctx context.Context, ev Event) error {
	data, err := json.Marshal(ev)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	send := func() error {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.url, bytes.NewReader(data))
		if err != nil {
			return fmt.Errorf("create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := s.client.Do(req)
		if err != nil {
			return fmt.Errorf("send event: %w", err)
		}
		defer func() {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
		}()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return &httpStatusError{code: resp.StatusCode}
		}
		return nil
	}

	return retry.Do(ctx, send, isRetriableHTTP)
}

// isRetriableHTTP возвращает true для ошибок, при которых повтор имеет смысл:
// сетевые/транспортные сбои и статусы 5xx или 429. Ошибки контекста и
// постоянные клиентские статусы (4xx, кроме 429) не повторяются.
func isRetriableHTTP(err error) bool {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	var se *httpStatusError
	if errors.As(err, &se) {
		return se.code == http.StatusTooManyRequests || se.code >= 500
	}
	// Ошибка транспорта (DNS, отказ соединения, таймаут клиента) — повторяемо.
	return true
}
