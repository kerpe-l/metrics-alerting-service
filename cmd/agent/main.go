package main

import (
	"log"
	"time"

	"github.com/kerpe-l/metrics-alerting-service/internal/agent"
)

const (
	pollInterval   = 2 * time.Second
	reportInterval = 10 * time.Second
	serverAddr     = "http://localhost:8080"
)

func main() {
	s := agent.NewStats()

	pollTicker := time.NewTicker(pollInterval)
	reportTicker := time.NewTicker(reportInterval)

	defer pollTicker.Stop()
	defer reportTicker.Stop()

	log.Println("Агент запущен...")

	for {
		select {
		case <-pollTicker.C:
			s.Collect()
			log.Println("Метрики собраны")

		case <-reportTicker.C:
			log.Println("Отправка метрик...")
			s.Send(serverAddr)
		}
	}
}
