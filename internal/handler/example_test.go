package handler_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/kerpe-l/metrics-alerting-service/internal/handler"
	"github.com/kerpe-l/metrics-alerting-service/internal/repository"
	"github.com/kerpe-l/metrics-alerting-service/internal/service"
)

// newExampleServer поднимает httptest-сервер с роутингом эндпоинтов
// практического трека поверх in-memory хранилища.
func newExampleServer() *httptest.Server {
	svc := service.NewMetricsService(repository.NewMemStorage())
	h := &handler.MetricsHandler{Service: svc}

	r := chi.NewRouter()
	r.Post("/update/{type}/{name}/{value}", h.UpdateHandler)
	r.Get("/value/{type}/{name}", h.ValueHandler)
	r.Post("/update/", h.UpdateJSONHandler)
	r.Post("/value/", h.ValueJSONHandler)
	r.Post("/updates/", h.UpdateBatchHandler)
	r.Get("/ping", h.PingDB)
	return httptest.NewServer(r)
}

// ExampleMetricsHandler_UpdateHandler демонстрирует обновление gauge-метрики
// через URL: POST /update/{type}/{name}/{value}.
func ExampleMetricsHandler_UpdateHandler() {
	ts := newExampleServer()
	defer ts.Close()

	resp, err := http.Post(ts.URL+"/update/gauge/Alloc/42.5", "text/plain", nil)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer func() { _ = resp.Body.Close() }()

	fmt.Println(resp.StatusCode)
	// Output:
	// 200
}

// ExampleMetricsHandler_ValueHandler демонстрирует чтение значения метрики
// текстом: GET /value/{type}/{name}.
func ExampleMetricsHandler_ValueHandler() {
	ts := newExampleServer()
	defer ts.Close()

	// Сначала записываем метрику.
	seed, err := http.Post(ts.URL+"/update/counter/Requests/10", "text/plain", nil)
	if err != nil {
		fmt.Println(err)
		return
	}
	_ = seed.Body.Close()

	resp, err := http.Get(ts.URL + "/value/counter/Requests")
	if err != nil {
		fmt.Println(err)
		return
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)
	fmt.Printf("%d %s\n", resp.StatusCode, body)
	// Output:
	// 200 10
}

// ExampleMetricsHandler_UpdateJSONHandler демонстрирует обновление метрики
// JSON-телом: POST /update/. counter-значения накапливаются.
func ExampleMetricsHandler_UpdateJSONHandler() {
	ts := newExampleServer()
	defer ts.Close()

	body := `{"id":"PollCount","type":"counter","delta":5}`
	resp, err := http.Post(ts.URL+"/update/", "application/json", strings.NewReader(body))
	if err != nil {
		fmt.Println(err)
		return
	}
	defer func() { _ = resp.Body.Close() }()

	out, _ := io.ReadAll(resp.Body)
	fmt.Print(string(out))
	// Output:
	// {"id":"PollCount","type":"counter","delta":5}
}

// ExampleMetricsHandler_ValueJSONHandler демонстрирует чтение метрики
// JSON-телом: POST /value/ с заполненными id и type.
func ExampleMetricsHandler_ValueJSONHandler() {
	ts := newExampleServer()
	defer ts.Close()

	seed, err := http.Post(ts.URL+"/update/",
		"application/json",
		strings.NewReader(`{"id":"Temp","type":"gauge","value":36.6}`))
	if err != nil {
		fmt.Println(err)
		return
	}
	_ = seed.Body.Close()

	resp, err := http.Post(ts.URL+"/value/",
		"application/json",
		strings.NewReader(`{"id":"Temp","type":"gauge"}`))
	if err != nil {
		fmt.Println(err)
		return
	}
	defer func() { _ = resp.Body.Close() }()

	out, _ := io.ReadAll(resp.Body)
	fmt.Print(string(out))
	// Output:
	// {"id":"Temp","type":"gauge","value":36.6}
}

// ExampleMetricsHandler_UpdateBatchHandler демонстрирует батч-обновление
// нескольких метрик одним запросом: POST /updates/.
func ExampleMetricsHandler_UpdateBatchHandler() {
	ts := newExampleServer()
	defer ts.Close()

	batch := `[{"id":"Alloc","type":"gauge","value":1.5},` +
		`{"id":"PollCount","type":"counter","delta":3}]`
	resp, err := http.Post(ts.URL+"/updates/", "application/json", bytes.NewBufferString(batch))
	if err != nil {
		fmt.Println(err)
		return
	}
	defer func() { _ = resp.Body.Close() }()

	fmt.Println(resp.StatusCode)
	// Output:
	// 200
}

// ExampleMetricsHandler_PingDB демонстрирует проверку соединения с БД:
// GET /ping. На in-memory хранилище БД отсутствует, поэтому ответ 500.
func ExampleMetricsHandler_PingDB() {
	ts := newExampleServer()
	defer ts.Close()

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, ts.URL+"/ping", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer func() { _ = resp.Body.Close() }()

	fmt.Println(resp.StatusCode)
	// Output:
	// 500
}
