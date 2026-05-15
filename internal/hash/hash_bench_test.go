package hash

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

func BenchmarkCompute(b *testing.B) {
	data := bytes.Repeat([]byte("metric-payload-"), 64) // ~1 KiB
	key := "secret-key"

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = Compute(data, key)
	}
}

func BenchmarkMiddleware(b *testing.B) {
	key := "secret-key"
	body := bytes.Repeat([]byte("payload"), 128)
	sum := Compute(body, key)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
	mw := Middleware(key)(handler)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
		req.Header.Set("HashSHA256", sum)
		w := httptest.NewRecorder()
		mw.ServeHTTP(w, req)
	}
}
