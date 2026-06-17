package crypto

import (
	"bytes"
	"crypto/rsa"
	"io"
	"net/http"
)

// HeaderEncrypted — заголовок-маркер, которым агент помечает зашифрованное тело.
const HeaderEncrypted = "X-Encrypted"

// Middleware расшифровывает тело запроса приватным ключом priv.
//
// Расшифровка применяется только к запросам с заголовком HeaderEncrypted —
// это позволяет незашифрованным запросам (например, /ping) проходить насквозь.
// При priv == nil middleware прозрачен. Размещать его нужно снаружи gzip-middleware:
// порядок на сервере — decrypt → gunzip → verify hash.
func Middleware(priv *rsa.PrivateKey) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if priv == nil || r.Header.Get(HeaderEncrypted) == "" {
				next.ServeHTTP(w, r)
				return
			}

			body, err := io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, "failed to read body", http.StatusInternalServerError)
				return
			}
			_ = r.Body.Close()

			plaintext, err := Decrypt(priv, body)
			if err != nil {
				http.Error(w, "decryption failed", http.StatusBadRequest)
				return
			}

			// Подменяем тело расшифрованными данными для последующих middleware/обработчиков.
			r.Body = io.NopCloser(bytes.NewReader(plaintext))
			r.ContentLength = int64(len(plaintext))
			next.ServeHTTP(w, r)
		})
	}
}
