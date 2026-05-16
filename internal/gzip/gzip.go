// Package gzip предоставляет middleware для прозрачной декомпрессии входящих
// gzip-запросов и сжатия ответов клиентам, поддерживающим gzip.
package gzip

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"
	"sync"
)

// writerPool переиспользует gzip.Writer между запросами,
// чтобы не аллоцировать flate.Writer на каждый ответ.
var writerPool = sync.Pool{
	New: func() any {
		// io.Discard — будет переустановлен через Reset перед использованием.
		return gzip.NewWriter(io.Discard)
	},
}

// readerPool переиспользует gzip.Reader для входящих сжатых запросов.
var readerPool = sync.Pool{
	New: func() any {
		// Возвращаем nil; настоящий reader создадим лениво — для Reset нужен поток данных.
		return (*gzip.Reader)(nil)
	},
}

type gzipWriter struct {
	http.ResponseWriter
	gz      *gzip.Writer
	started bool
}

func (w *gzipWriter) decide() {
	if w.started {
		return
	}
	w.started = true
	ct := w.Header().Get("Content-Type")
	if strings.HasPrefix(ct, "application/json") || strings.HasPrefix(ct, "text/html") {
		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Del("Content-Length")
		gz := writerPool.Get().(*gzip.Writer)
		gz.Reset(w.ResponseWriter)
		w.gz = gz
	}
}

func (w *gzipWriter) WriteHeader(code int) {
	w.decide()
	w.ResponseWriter.WriteHeader(code)
}

func (w *gzipWriter) Write(b []byte) (int, error) {
	w.decide()
	if w.gz != nil {
		return w.gz.Write(b)
	}
	return w.ResponseWriter.Write(b)
}

func (w *gzipWriter) Close() error {
	if w.gz == nil {
		return nil
	}
	err := w.gz.Close()
	writerPool.Put(w.gz)
	w.gz = nil
	return err
}

// Middleware обрабатывает входящие gzip-запросы и сжимает ответы для клиентов, поддерживающих gzip
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Декомпрессия запроса
		if r.Header.Get("Content-Encoding") == "gzip" {
			gz, _ := readerPool.Get().(*gzip.Reader)
			var err error
			if gz == nil {
				gz, err = gzip.NewReader(r.Body)
			} else {
				err = gz.Reset(r.Body)
			}
			if err != nil {
				if gz != nil {
					readerPool.Put(gz)
				}
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			origBody := r.Body
			r.Body = struct {
				io.Reader
				io.Closer
			}{gz, r.Body}
			defer func() {
				_ = gz.Close()
				readerPool.Put(gz)
				_ = origBody.Close()
			}()
		}

		// Компрессия ответа.
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			next.ServeHTTP(w, r)
			return
		}

		gw := &gzipWriter{ResponseWriter: w}
		defer func() { _ = gw.Close() }()
		next.ServeHTTP(gw, r)
	})
}
