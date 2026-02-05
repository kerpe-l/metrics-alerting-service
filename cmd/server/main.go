package main

import (
	"flag"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/kerpe-l/metrics-alerting-service/internal/handler"
	"github.com/kerpe-l/metrics-alerting-service/internal/repository"
)

func main() {
	addr := flag.String("a", "localhost:8080", "address and port to run server")

	flag.Parse()

	storage := repository.NewMemStorage()
	h := &handler.MetricsHandler{Storage: storage}

	r := chi.NewRouter()
	r.Use(middleware.Logger)

	r.Get("/", h.RootHandler)
	r.Post("/update/{type}/{name}/{value}", h.UpdateHandler)
	r.Get("/value/{type}/{name}", h.ValueHandler)

	log.Printf("Сервер запущен на %s\n", *addr)

	err := http.ListenAndServe(*addr, r)
	if err != nil {
		log.Fatal("Ошибка запуска сервера: ", err)
	}
}
