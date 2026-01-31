package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/kerpe-l/metrics-alerting-service/internal/handler"
	"github.com/kerpe-l/metrics-alerting-service/internal/repository"
)

func main() {
	storage := repository.NewMemStorage()
	h := &handler.MetricsHandler{Storage: storage}

	http.HandleFunc("/update/", h.UpdateHandler)

	// TODO удалить потом
	http.HandleFunc("/debug", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Counters: %v\nGauges: %v", storage.Counters, storage.Gauges)
	})

	log.Println("Сервер запущен на http://localhost:8080")

	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatal("Ошибка запуска сервера: ", err)
	}
}
