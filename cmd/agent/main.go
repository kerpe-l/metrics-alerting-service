package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/kerpe-l/metrics-alerting-service/internal/agent"
)

func main() {
	addr := flag.String("a", "localhost:8080", "address and port of metrics server")
	reportInterval := flag.Int("r", 10, "report interval in seconds")
	pollInterval := flag.Int("p", 2, "poll interval in seconds")

	flag.Parse()

	if envAddr := os.Getenv("ADDRESS"); envAddr != "" {
		*addr = envAddr
	}
	if envReport := os.Getenv("REPORT_INTERVAL"); envReport != "" {
		if v, err := strconv.Atoi(envReport); err == nil {
			*reportInterval = v
		}
	}
	if envPoll := os.Getenv("POLL_INTERVAL"); envPoll != "" {
		if v, err := strconv.Atoi(envPoll); err == nil {
			*pollInterval = v
		}
	}

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
