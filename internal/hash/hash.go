// Package hash вычисляет и проверяет подпись HMAC-SHA256 тела запроса/ответа
// и предоставляет middleware для прозрачной проверки и подписи.
package hash

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"hash"
	"sync"
)

// hasherPool кэширует пары (key → hash.Hash) для переиспользования между запросами.
// Hash.Reset позволяет переиспользовать аллокации внутреннего состояния sha256.
var hasherPool sync.Map // map[string]*sync.Pool

func getHasher(key string) hash.Hash {
	p, ok := hasherPool.Load(key)
	if !ok {
		k := []byte(key)
		newPool := &sync.Pool{
			New: func() any { return hmac.New(sha256.New, k) },
		}
		p, _ = hasherPool.LoadOrStore(key, newPool)
	}
	return p.(*sync.Pool).Get().(hash.Hash)
}

func putHasher(key string, h hash.Hash) {
	if p, ok := hasherPool.Load(key); ok {
		h.Reset()
		p.(*sync.Pool).Put(h)
	}
}

// Compute вычисляет HMAC-SHA256 от data с указанным ключом.
// Возвращает hex-строку хеша. Использует пул hash.Hash для уменьшения аллокаций.
func Compute(data []byte, key string) string {
	h := getHasher(key)
	defer putHasher(key, h)

	h.Write(data)
	var buf [sha256.Size]byte
	sum := h.Sum(buf[:0])
	return hex.EncodeToString(sum)
}

// Verify проверяет, что expected совпадает с вычисленным HMAC-SHA256 от data.
func Verify(data []byte, key, expected string) bool {
	computed := Compute(data, key)
	return hmac.Equal([]byte(computed), []byte(expected))
}
