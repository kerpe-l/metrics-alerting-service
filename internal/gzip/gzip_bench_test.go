package gzip

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

func BenchmarkMiddleware_CompressJSON(b *testing.B) {
	payload := bytes.Repeat([]byte(`{"id":"x","type":"gauge","value":3.14},`), 64)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(payload)
	})
	mw := Middleware(handler)

	b.ReportAllocs()
	for b.Loop() {
		b.StopTimer()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Accept-Encoding", "gzip")
		w := httptest.NewRecorder()
		b.StartTimer()

		mw.ServeHTTP(w, req)
	}
}
