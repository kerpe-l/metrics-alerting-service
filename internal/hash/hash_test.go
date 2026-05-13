package hash

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCompute(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		key      string
		wantSame string // если не пусто — ожидаем равенство с этим значением
	}{
		{
			name: "deterministic",
			data: []byte("hello"),
			key:  "secret",
		},
		{
			name: "empty data",
			data: []byte(""),
			key:  "secret",
		},
		{
			name: "empty key",
			data: []byte("hello"),
			key:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h1 := Compute(tt.data, tt.key)
			h2 := Compute(tt.data, tt.key)
			assert.Equal(t, h1, h2, "одинаковые входные данные должны давать одинаковый хеш")
			assert.Len(t, h1, 64, "HMAC-SHA256 hex = 64 символа")
		})
	}

	// Разные данные / ключи → разные хеши.
	t.Run("different data produces different hash", func(t *testing.T) {
		assert.NotEqual(t, Compute([]byte("hello"), "secret"), Compute([]byte("world"), "secret"))
	})
	t.Run("different key produces different hash", func(t *testing.T) {
		assert.NotEqual(t, Compute([]byte("hello"), "secret"), Compute([]byte("hello"), "other"))
	})
}

func TestVerify(t *testing.T) {
	data := []byte(`{"id":"test","type":"gauge","value":1.23}`)
	key := "mykey"
	validHash := Compute(data, key)

	tests := []struct {
		name     string
		data     []byte
		key      string
		hash     string
		wantOK   bool
	}{
		{
			name:   "valid hash",
			data:   data,
			key:    key,
			hash:   validHash,
			wantOK: true,
		},
		{
			name:   "invalid hash string",
			data:   data,
			key:    key,
			hash:   "invalidhash",
			wantOK: false,
		},
		{
			name:   "wrong key",
			data:   data,
			key:    "wrongkey",
			hash:   validHash,
			wantOK: false,
		},
		{
			name:   "wrong data",
			data:   []byte("other data"),
			key:    key,
			hash:   validHash,
			wantOK: false,
		},
		{
			name:   "empty hash",
			data:   data,
			key:    key,
			hash:   "",
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.wantOK, Verify(tt.data, tt.key, tt.hash))
		})
	}
}

func TestMiddleware(t *testing.T) {
	const key = "testkey"
	body := `[{"id":"Alloc","type":"gauge","value":100}]`
	respBody := `{"status":"ok"}`

	// Эталонный хендлер: читает тело, пишет ответ.
	echoHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		assert.Equal(t, body, string(b))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(respBody))
	})

	tests := []struct {
		name           string
		key            string
		hashHeader     string // значение заголовка HashSHA256 (пусто = не ставим)
		wantStatus     int
		wantRespHash   bool // ожидаем ли HashSHA256 в ответе
		wantHandlerHit bool // должен ли вызваться хендлер
	}{
		{
			name:           "no key — pass-through",
			key:            "",
			hashHeader:     "",
			wantStatus:     http.StatusOK,
			wantRespHash:   false,
			wantHandlerHit: true,
		},
		{
			name:           "valid hash — 200 + response signed",
			key:            key,
			hashHeader:     Compute([]byte(body), key),
			wantStatus:     http.StatusOK,
			wantRespHash:   true,
			wantHandlerHit: true,
		},
		{
			name:           "invalid hash — 400",
			key:            key,
			hashHeader:     "badhash",
			wantStatus:     http.StatusBadRequest,
			wantRespHash:   false,
			wantHandlerHit: false,
		},
		{
			name:           "wrong key hash — 400",
			key:            key,
			hashHeader:     Compute([]byte(body), "wrongkey"),
			wantStatus:     http.StatusBadRequest,
			wantRespHash:   false,
			wantHandlerHit: false,
		},
		{
			name:           "key set, no header — pass-through + response signed",
			key:            key,
			hashHeader:     "",
			wantStatus:     http.StatusOK,
			wantRespHash:   true,
			wantHandlerHit: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handlerHit := false
			inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handlerHit = true
				echoHandler.ServeHTTP(w, r)
			})

			handler := Middleware(tt.key)(inner)

			req := httptest.NewRequest(http.MethodPost, "/updates/", strings.NewReader(body))
			if tt.hashHeader != "" {
				req.Header.Set("HashSHA256", tt.hashHeader)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			assert.Equal(t, tt.wantStatus, rec.Code)
			assert.Equal(t, tt.wantHandlerHit, handlerHit)

			if tt.wantRespHash {
				respHash := rec.Header().Get("HashSHA256")
				assert.NotEmpty(t, respHash)
				assert.True(t, Verify(rec.Body.Bytes(), tt.key, respHash),
					"хеш ответа должен быть валидным")
			} else {
				assert.Empty(t, rec.Header().Get("HashSHA256"))
			}
		})
	}
}
