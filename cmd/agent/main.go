package main

import (
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/kerpe-l/metrics-alerting-service/internal/agent"
)

func main() {
	addr := flag.String("a", "localhost:8080", "address and port of metrics server")
	reportInterval := flag.Int("r", 10, "report interval in seconds")
	pollInterval := flag.Int("p", 2, "poll interval in seconds")

	flag.Parse()

	reportDuration := time.Duration(*reportInterval) * time.Second
	pollDuration := time.Duration(*pollInterval) * time.Second

	serverAddr := fmt.Sprintf("http://%s", *addr)

	s := agent.NewStats()

	pollTicker := time.NewTicker(pollDuration)
	reportTicker := time.NewTicker(reportDuration)

	defer pollTicker.Stop()
	defer reportTicker.Stop()

	log.Printf("Агент запущен. Poll: %v, Report: %v, Server: %s\n", pollDuration, reportDuration, serverAddr)

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
