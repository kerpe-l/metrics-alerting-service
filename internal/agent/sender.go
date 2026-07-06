package agent

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/kerpe-l/metrics-alerting-service/internal/crypto"
	"github.com/kerpe-l/metrics-alerting-service/internal/hash"
	"github.com/kerpe-l/metrics-alerting-service/internal/logger"
	"github.com/kerpe-l/metrics-alerting-service/internal/model"
	"github.com/kerpe-l/metrics-alerting-service/internal/retry"
)

// gzipWriterPool переиспользует gzip.Writer между вызовами Send.
var gzipWriterPool = sync.Pool{
	New: func() any {
		gz, _ := gzip.NewWriterLevel(io.Discard, gzip.BestSpeed)
		return gz
	},
}

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
	key        string
	realIP     string
	pubKey     *rsa.PublicKey
	client     *http.Client
}

// NewSender создаёт новый отправитель метрик. realIP проставляется в заголовок
// X-Real-IP каждого запроса (пусто = заголовок не ставится).
// При pubKey != nil тело каждого запроса шифруется публичным ключом сервера.
func NewSender(serverAddr, key, realIP string, pubKey *rsa.PublicKey) *Sender {
	return &Sender{
		serverAddr: serverAddr,
		key:        key,
		realIP:     realIP,
		pubKey:     pubKey,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Send отправляет слайс метрик батчем на /updates/.
func (s *Sender) Send(ctx context.Context, metrics []model.Metrics) {
	if len(metrics) == 0 {
		return
	}

	data, err := json.Marshal(metrics)
	if err != nil {
		logger.Log.Error("failed to marshal", zap.Error(err))
		return
	}

	// Сжимаем запрос через gzip, переиспользуя writer из пула.
	var buf bytes.Buffer
	buf.Grow(len(data) / 2)
	gz := gzipWriterPool.Get().(*gzip.Writer)
	gz.Reset(&buf)
	if _, err = gz.Write(data); err != nil {
		gzipWriterPool.Put(gz)
		logger.Log.Error("failed to write gzip data", zap.Error(err))
		return
	}
	if err = gz.Close(); err != nil {
		gzipWriterPool.Put(gz)
		logger.Log.Error("failed to close gzip writer", zap.Error(err))
		return
	}
	gzipWriterPool.Put(gz)

	body := buf.Bytes()

	// Шифруем сжатое тело публичным ключом сервера, если шифрование включено.
	encrypted := false
	if s.pubKey != nil {
		enc, encErr := crypto.Encrypt(s.pubKey, body)
		if encErr != nil {
			logger.Log.Error("failed to encrypt body", zap.Error(encErr))
			return
		}
		body = enc
		encrypted = true
	}

	endpoint := s.serverAddr + "/updates/"

	err = retry.Do(ctx, func() error {
		req, reqErr := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
		if reqErr != nil {
			return reqErr
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Content-Encoding", "gzip")
		req.Header.Set("Accept-Encoding", "gzip")
		if s.realIP != "" {
			req.Header.Set("X-Real-IP", s.realIP)
		}
		if encrypted {
			req.Header.Set(crypto.HeaderEncrypted, "1")
		}
		if s.key != "" {
			req.Header.Set("HashSHA256", hash.Compute(data, s.key))
		}

		resp, doErr := s.client.Do(req)
		if doErr != nil {
			return doErr
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode >= http.StatusInternalServerError {
			body, readErr := io.ReadAll(resp.Body)
			if readErr != nil {
				return fmt.Errorf("%w: %d (failed to read body: %v)", errServerUnavailable, resp.StatusCode, readErr)
			}
			return fmt.Errorf("%w: %d %s", errServerUnavailable, resp.StatusCode, bytes.TrimSpace(body))
		}
		if resp.StatusCode >= http.StatusBadRequest {
			body, readErr := io.ReadAll(resp.Body)
			if readErr != nil {
				return fmt.Errorf("request rejected: %d (failed to read body: %v)", resp.StatusCode, readErr)
			}
			return fmt.Errorf("request rejected: %d %s", resp.StatusCode, bytes.TrimSpace(body))
		}
		return nil
	}, isRetriableHTTPError)
	if err != nil {
		logger.Log.Error("failed to send metrics", zap.Error(err))
	}
}
