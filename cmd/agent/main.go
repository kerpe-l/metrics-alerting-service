package main

import (
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/kerpe-l/metrics-alerting-service/internal/agent"
	"github.com/kerpe-l/metrics-alerting-service/internal/config"
	"github.com/kerpe-l/metrics-alerting-service/internal/logger"
)

func main() {
	cfg := config.NewAgentConfig()

	if err := logger.Initialize("info"); err != nil {
		panic(err)
	}

	reportDuration := time.Duration(cfg.ReportInterval) * time.Second
	pollDuration := time.Duration(cfg.PollInterval) * time.Second

	serverAddr := fmt.Sprintf("http://%s", cfg.Address)

	collector := agent.NewCollector()
	sender := agent.NewSender(serverAddr, cfg.Key)

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
			collector.Collect()
			logger.Log.Info("Метрики собраны")

		case <-reportTicker.C:
			logger.Log.Info("Отправка метрик...")
			sender.Send(collector.Metrics())
		}
	}
}
