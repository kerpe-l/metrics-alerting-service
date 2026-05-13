package hash

import (
	"bytes"
	"io"
	"net/http"

	"github.com/kerpe-l/metrics-alerting-service/internal/logger"
)

const headerName = "HashSHA256"

// hashResponseWriter перехватывает тело ответа для вычисления хеша.
type hashResponseWriter struct {
	http.ResponseWriter
	buf        bytes.Buffer
	statusCode int
	wroteHeader bool
}

func (w *hashResponseWriter) WriteHeader(code int) {
	w.statusCode = code
	w.wroteHeader = true
}

func (w *hashResponseWriter) Write(b []byte) (int, error) {
	return w.buf.Write(b)
}

// Middleware проверяет HMAC-SHA256 входящего запроса и подписывает ответ.
// При пустом ключе middleware является прозрачным pass-through.
func Middleware(key string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if key == "" {
				next.ServeHTTP(w, r)
				return
			}

			// Проверяем хеш входящего запроса, если заголовок присутствует.
			if receivedHash := r.Header.Get(headerName); receivedHash != "" {
				body, err := io.ReadAll(r.Body)
				if err != nil {
					http.Error(w, "failed to read body", http.StatusInternalServerError)
					return
				}
				_ = r.Body.Close()

				if !Verify(body, key, receivedHash) {
					http.Error(w, "hash mismatch", http.StatusBadRequest)
					return
				}

				// Восстанавливаем тело запроса для последующих обработчиков.
				r.Body = io.NopCloser(bytes.NewReader(body))
			}

			// Перехватываем ответ для вычисления хеша.
			hw := &hashResponseWriter{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
			}
			next.ServeHTTP(hw, r)

			// Вычисляем хеш тела ответа и устанавливаем заголовок.
			respBody := hw.buf.Bytes()
			if len(respBody) > 0 {
				w.Header().Set(headerName, Compute(respBody, key))
			}

			if hw.wroteHeader {
				w.WriteHeader(hw.statusCode)
			}
			if len(respBody) > 0 {
				if _, err := w.Write(respBody); err != nil {
					logger.Log.Error("ошибка записи ответа: " + err.Error())
				}
			}
		})
	}
}
