package agent

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"go.uber.org/zap"

	"github.com/kerpe-l/metrics-alerting-service/internal/logger"
	"github.com/kerpe-l/metrics-alerting-service/internal/model"
	"github.com/kerpe-l/metrics-alerting-service/internal/retry"
)

// errServerUnavailable — ошибка для 5xx ответов сервера.
var errServerUnavailable = errors.New("server unavailable")

// isRetriableHTTPError проверяет, стоит ли повторять HTTP-запрос.
// Retriable: сетевые ошибки (*url.Error) и 5xx ответы сервера.
func isRetriableHTTPError(err error) bool {
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return true
	}
	return errors.Is(err, errServerUnavailable)
}

// Sender отправляет метрики на сервер.
type Sender struct {
	serverAddr string
}

// NewSender создаёт новый отправитель метрик.
func NewSender(serverAddr string) *Sender {
	return &Sender{serverAddr: serverAddr}
}

// Send отправляет слайс метрик батчем на /updates/.
func (s *Sender) Send(metrics []model.Metrics) {
	if len(metrics) == 0 {
		return
	}

	data, err := json.Marshal(metrics)
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
	if _, err := gz.Write(data); err != nil {
		logger.Log.Info("failed to write gzip data", zap.Error(err))
		return
	}
	gz.Close()

	compressed := buf.Bytes()
	endpoint := s.serverAddr + "/updates/"

	err = retry.Do(func() error {
		req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(compressed))
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
			body, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("%w: %d %s", errServerUnavailable, resp.StatusCode, bytes.TrimSpace(body))
		}
		return nil
	}, isRetriableHTTPError)
	if err != nil {
		logger.Log.Info("failed to send metrics", zap.Error(err))
	}
}
