package main

import (
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/kerpe-l/metrics-alerting-service/internal/agent"
	"github.com/kerpe-l/metrics-alerting-service/internal/config"
	"github.com/kerpe-l/metrics-alerting-service/internal/logger"
	"github.com/kerpe-l/metrics-alerting-service/internal/model"
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

	logger.Log.Info("agent started",
		zap.Duration("poll", pollDuration),
		zap.Duration("report", reportDuration),
		zap.String("server", serverAddr),
		zap.Int("rateLimit", cfg.RateLimit),
	)

	// Канал для передачи метрик воркерам.
	jobs := make(chan []model.Metrics, cfg.RateLimit)

	// Worker pool: ограничиваем количество одновременных исходящих запросов.
	for i := 0; i < cfg.RateLimit; i++ {
		go func() {
			for metrics := range jobs {
				sender.Send(metrics)
			}
		}()
	}

	// Горутина сбора runtime-метрик.
	go func() {
		pollTicker := time.NewTicker(pollDuration)
		defer pollTicker.Stop()
		for range pollTicker.C {
			collector.Collect()
			logger.Log.Info("Метрики собраны")
		}
	}()

	// Горутина сбора дополнительных метрик (gopsutil).
	go func() {
		pollTicker := time.NewTicker(pollDuration)
		defer pollTicker.Stop()
		for range pollTicker.C {
			collector.CollectExtra()
			logger.Log.Info("Дополнительные метрики собраны")
		}
	}()

	// Горутина отправки метрик.
	reportTicker := time.NewTicker(reportDuration)
	defer reportTicker.Stop()
	for range reportTicker.C {
		logger.Log.Info("Отправка метрик...")
		jobs <- collector.Metrics()
	}
}
