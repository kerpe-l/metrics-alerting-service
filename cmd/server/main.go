package main

import (
	"flag"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/kerpe-l/metrics-alerting-service/internal/handler"
	"github.com/kerpe-l/metrics-alerting-service/internal/logger"
	"github.com/kerpe-l/metrics-alerting-service/internal/repository"
)

func main() {
	addr := flag.String("a", "localhost:8080", "address and port to run server")

	flag.Parse()

	if envAddr := os.Getenv("ADDRESS"); envAddr != "" {
		*addr = envAddr
	}

	if err := logger.Initialize("info"); err != nil {
		panic(err)
	}

	storage := repository.NewMemStorage()
	h := &handler.MetricsHandler{Storage: storage}

	r := chi.NewRouter()
	r.Use(logger.RequestLogger)

	r.Get("/", h.RootHandler)
	r.Post("/update/{type}/{name}/{value}", h.UpdateHandler)
	r.Get("/value/{type}/{name}", h.ValueHandler)
	r.Post("/update/", h.UpdateJSONHandler)
	r.Post("/value/", h.ValueJSONHandler)

	logger.Log.Info("Сервер запущен на " + *addr)

	err := http.ListenAndServe(*addr, r)
	if err != nil {
		logger.Log.Fatal(err.Error())
	}
}
