package gzip

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// echo handler — отвечает JSON тем, что пришло в теле запроса.
func echoHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = io.Copy(w, r.Body)
}

func TestMiddleware_CompressesJSONResponse(t *testing.T) {
	mw := Middleware(http.HandlerFunc(echoHandler))

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"a":1}`))
	req.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()
	mw.ServeHTTP(w, req)

	resp := w.Result()
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, "gzip", resp.Header.Get("Content-Encoding"))

	gz, err := gzip.NewReader(resp.Body)
	require.NoError(t, err)
	body, err := io.ReadAll(gz)
	require.NoError(t, err)
	assert.Equal(t, `{"a":1}`, string(body))
}

func TestMiddleware_PassThroughPlain(t *testing.T) {
	mw := Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("hello"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	w := httptest.NewRecorder()
	mw.ServeHTTP(w, req)

	resp := w.Result()
	defer func() { _ = resp.Body.Close() }()

	assert.Equal(t, "", resp.Header.Get("Content-Encoding"))
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, "hello", string(body))
}

func TestMiddleware_DecompressesRequest(t *testing.T) {
	mw := Middleware(http.HandlerFunc(echoHandler))

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	_, _ = gz.Write([]byte(`{"hello":"world"}`))
	require.NoError(t, gz.Close())

	req := httptest.NewRequest(http.MethodPost, "/", &buf)
	req.Header.Set("Content-Encoding", "gzip")
	w := httptest.NewRecorder()
	mw.ServeHTTP(w, req)

	resp := w.Result()
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, `{"hello":"world"}`, string(body))
}

func TestMiddleware_BadGzipBody(t *testing.T) {
	mw := Middleware(http.HandlerFunc(echoHandler))

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("not gzip"))
	req.Header.Set("Content-Encoding", "gzip")
	w := httptest.NewRecorder()
	mw.ServeHTTP(w, req)

	resp := w.Result()
	defer func() { _ = resp.Body.Close() }()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestMiddleware_NoAcceptEncoding(t *testing.T) {
	mw := Middleware(http.HandlerFunc(echoHandler))

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"a":1}`))
	w := httptest.NewRecorder()
	mw.ServeHTTP(w, req)

	resp := w.Result()
	defer func() { _ = resp.Body.Close() }()
	assert.Equal(t, "", resp.Header.Get("Content-Encoding"))
}
