package main

import (
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/kerpe-l/metrics-alerting-service/internal/handler"
	"github.com/kerpe-l/metrics-alerting-service/internal/repository"
)

func main() {
	storage := repository.NewMemStorage()
	h := &handler.MetricsHandler{Storage: storage}

	r := chi.NewRouter()

	r.Use(middleware.Logger)

	r.Get("/", h.RootHandler)
	r.Post("/update/{type}/{name}/{value}", h.UpdateHandler)
	r.Get("/value/{type}/{name}", h.ValueHandler)

	log.Println("Сервер запущен на http://localhost:8080")

	err := http.ListenAndServe(":8080", r)
	if err != nil {
		log.Fatal("Ошибка запуска сервера: ", err)
	}
}
