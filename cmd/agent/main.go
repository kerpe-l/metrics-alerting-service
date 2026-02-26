package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"time"

	"go.uber.org/zap"

	"github.com/kerpe-l/metrics-alerting-service/internal/agent"
	"github.com/kerpe-l/metrics-alerting-service/internal/logger"
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

	if err := logger.Initialize("info"); err != nil {
		panic(err)
	}

	reportDuration := time.Duration(*reportInterval) * time.Second
	pollDuration := time.Duration(*pollInterval) * time.Second

	serverAddr := fmt.Sprintf("http://%s", *addr)

	s := agent.NewStats()

	pollTicker := time.NewTicker(pollDuration)
	reportTicker := time.NewTicker(reportDuration)

	defer pollTicker.Stop()
	defer reportTicker.Stop()

	logger.Log.Info("agent started",
		zap.Duration("poll", pollDuration),
		zap.Duration("report", reportDuration),
		zap.String("server", serverAddr),
	)

	for {
		select {
		case <-pollTicker.C:
			s.Collect()
			logger.Log.Info("Метрики собраны")

		case <-reportTicker.C:
			logger.Log.Info("Отправка метрик...")
			s.Send(serverAddr)
		}
	}
}
