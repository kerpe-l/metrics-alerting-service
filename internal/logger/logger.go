// Package logger предоставляет глобальный zap-логгер и middleware
// логирования HTTP-запросов и ответов.
package logger

import (
	"net/http"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Log — глобальный логгер. До вызова Initialize это no-op логгер.
var Log *zap.Logger = zap.NewNop()

// Initialize настраивает глобальный Log на production-конфигурацию zap
// с указанным уровнем (например, "info", "debug"). Возвращает ошибку,
// если уровень не распознан или логгер не удалось собрать.
func Initialize(level string) error {
	lvl, err := zap.ParseAtomicLevel(level)
	if err != nil {
		return err
	}

	cfg := zap.NewProductionConfig()
	cfg.Level = lvl
	cfg.EncoderConfig.TimeKey = "ts"
	cfg.EncoderConfig.EncodeTime = zapcore.RFC3339TimeEncoder

	zl, err := cfg.Build()
	if err != nil {
		return err
	}

	Log = zl
	return nil
}

type responseData struct {
	status int
	size   int
}

type loggingResponseWriter struct {
	http.ResponseWriter
	responseData *responseData
}

func (r *loggingResponseWriter) Write(b []byte) (int, error) {
	size, err := r.ResponseWriter.Write(b)
	r.responseData.size += size
	return size, err
}

func (r *loggingResponseWriter) WriteHeader(statusCode int) {
	r.ResponseWriter.WriteHeader(statusCode)
	r.responseData.status = statusCode
}

// RequestLogger — middleware, логирующий метод, URI и длительность запроса,
// а также статус и размер ответа.
func RequestLogger(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		rd := &responseData{
			status: 200,
		}
		lw := loggingResponseWriter{
			ResponseWriter: w,
			responseData:   rd,
		}

		h.ServeHTTP(&lw, r)

		duration := time.Since(start)

		Log.Info("request",
			zap.String("uri", r.RequestURI),
			zap.String("method", r.Method),
			zap.Duration("duration", duration),
		)

		Log.Info("response",
			zap.Int("status", rd.status),
			zap.Int("size", rd.size),
		)
	})
}
