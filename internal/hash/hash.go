package hash

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
)

// Compute вычисляет HMAC-SHA256 от data с указанным ключом.
// Возвращает hex-строку хеша.
func Compute(data []byte, key string) string {
	h := hmac.New(sha256.New, []byte(key))
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}

// Verify проверяет, что expected совпадает с вычисленным HMAC-SHA256 от data.
func Verify(data []byte, key, expected string) bool {
	computed := Compute(data, key)
	return hmac.Equal([]byte(computed), []byte(expected))
}
